package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
	mongodb_opts "go.mongodb.org/mongo-driver/mongo/options"
)

func parseRepoName(repoName string) (namespace, name string) {
	parts := strings.SplitN(repoName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "library", repoName
}

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

type ParallelCrawler struct {
	WorkerCount int
	RepoChan    chan *myutils.Repository
	WG          sync.WaitGroup
	IM          *IdentityManager
	crawledKeys sync.Map // Rastreia keywords já feitas
	seenRepos   sync.Map // Deduplicação em RAM (igual ao Python)
}

func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	return &ParallelCrawler{
		WorkerCount: workers,
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
	}
}

func (pc *ParallelCrawler) loadState() {
	if !myutils.GlobalDBClient.MongoFlag {
		return
	}
	myutils.Logger.Info("Loading state from MongoDB...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	opts := mongodb_opts.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := myutils.GlobalDBClient.Mongo.KeywordsColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct{ ID string `bson:"_id"` }
		if err := cursor.Decode(&doc); err == nil {
			pc.crawledKeys.Store(doc.ID, true)
		}
	}
}

func (pc *ParallelCrawler) repoWriter(ctx context.Context, done chan struct{}) {
	defer close(done)
	buffer := make([]*myutils.Repository, 0, 1000)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		_ = myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer)
		myutils.Logger.Info(fmt.Sprintf("Flushed %d unique repos to MongoDB", len(buffer)))
		buffer = buffer[:0]
	}

	for {
		select {
		case repo, ok := <-pc.RepoChan:
			if !ok {
				flush()
				return
			}
			buffer = append(buffer, repo)
			if len(buffer) >= 1000 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (pc *ParallelCrawler) Start(seeds []string) {
	pc.loadState()
	myutils.Logger.Info(fmt.Sprintf("Starting Serial-Parallel Pipeline [W:%d]", pc.WorkerCount))

	writerDone := make(chan struct{})
	go pc.repoWriter(context.Background(), writerDone)

	// Distribui sementes iniciais entre os workers
	if len(seeds) == 0 {
		for _, ch := range alphabet {
			seeds = append(seeds, string(ch))
		}
	}

	// Canal local apenas para distribuir as sementes iniciais
	seedChan := make(chan string, len(seeds))
	for _, s := range seeds {
		seedChan <- s
	}
	close(seedChan)

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker(i, seedChan)
	}

	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	myutils.Logger.Info("Crawl Complete")
}

func (pc *ParallelCrawler) worker(id int, seedChan <-chan string) {
	defer pc.WG.Done()
	
	// Strict Account Isolation
	client, token := pc.IM.GetNextClient()
	if pc.WorkerCount <= len(pc.IM.Accounts) {
		acc := pc.IM.Accounts[id]
		if acc.Token == "" {
			pc.IM.LoginDockerHub(acc)
		}
		token = acc.Token
		client = &http.Client{Timeout: 20 * time.Second}
	}

	// Inicia DFS recursivo a partir das sementes
	for seed := range seedChan {
		client, token = pc.crawlDFS(seed, client, token)
	}
}

type V2SearchResponse struct {
	Count        int `json:"count"`
	Repositories []struct {
		RepoName  string `json:"repo_name"`
		PullCount int64  `json:"pull_count"`
	} `json:"results"`
}

// crawlDFS is the recursive DFS function, mirroring the Python logic exactly.
func (pc *ParallelCrawler) crawlDFS(prefix string, client *http.Client, token string) (*http.Client, string) {
	if _, crawled := pc.crawledKeys.Load(prefix); crawled {
		return client, token
	}

	myutils.Logger.Info(fmt.Sprintf("Prefix: [%s]", prefix))
	
	// Fetch Page 1 to get Total Count
	res, nextClient, nextToken := pc.fetchPage(prefix, 1, client, token)
	client, token = nextClient, nextToken
	
	if res == nil {
		return client, token // Falha de rede irrecuperável
	}

	total := res.Count
	pages := (total / 100) + 1
	if pages > 100 {
		pages = 100
	}

	// Processa a página 1 que já foi baixada
	pc.processResults(res.Repositories)

	// Scrape Sequencial Constante (A mágica do Python)
	for p := 2; p <= pages; p++ {
		time.Sleep(200 * time.Millisecond) // Delay de 0.2s igual ao Python
		
		resPage, nextC, nextT := pc.fetchPage(prefix, p, client, token)
		client, token = nextC, nextT
		
		if resPage == nil {
			break
		}
		pc.processResults(resPage.Repositories)
	}

	// DFS Collision Logic
	if total >= 10000 && len(prefix) < 255 {
		myutils.Logger.Info(fmt.Sprintf("Collision on '%s'. Deepening search space...", prefix))
		for _, char := range alphabet {
			client, token = pc.crawlDFS(prefix+string(char), client, token)
		}
	}

	// Save state
	pc.crawledKeys.Store(prefix, true)
	if myutils.GlobalDBClient.MongoFlag {
		go func(k string) {
			_ = myutils.GlobalDBClient.Mongo.MarkKeywordCrawled(k)
		}(prefix)
	}

	return client, token
}

func (pc *ParallelCrawler) fetchPage(query string, page int, client *http.Client, token string) (*V2SearchResponse, *http.Client, string) {
	url := myutils.GetV2SearchURL(query, page, 100)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" {
		req.Header.Add("Authorization", "JWT "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, client, token
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		myutils.Logger.Warn("Rate limited. Rotating identity and sleeping 10s...")
		time.Sleep(10 * time.Second) // Backoff menor que o Python (60s) pois temos rotação
		newClient, newToken := pc.IM.GetNextClient()
		return pc.fetchPage(query, page, newClient, newToken)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, client, token
	}

	var res V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, client, token
	}

	return &res, client, token
}

func (pc *ParallelCrawler) processResults(repositories []struct {
	RepoName  string "json:\"repo_name\""
	PullCount int64  "json:\"pull_count\""
}) {
	for _, r := range repositories {
		if r.RepoName == "" {
			continue
		}
		
		// 1. Deduplicação em Memória RAM (O(1)) - Idêntico ao Python
		if _, seen := pc.seenRepos.LoadOrStore(r.RepoName, true); seen {
			continue // Já vimos este repo, ignora (economiza rede e banco)
		}

		ns, name := parseRepoName(r.RepoName)
		pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: r.PullCount}
	}
}

func ShardSeeds(shard, total int) []string {
	chars := []rune(alphabet)
	n := len(chars)
	size := n / total
	start, end := shard*size, (shard+1)*size
	if shard == total-1 {
		end = n
	}
	var seeds []string
	for _, ch := range chars[start:end] {
		seeds = append(seeds, string(ch))
	}
	return seeds
}