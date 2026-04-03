package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
	mongodb_opts "go.mongodb.org/mongo-driver/mongo/options"
)

var pageConcurrency = func() int {
	if v := os.Getenv("PAGE_CONCURRENCY"); v != "" {
		var n int
		fmt.Sscan(v, &n)
		if n > 0 {
			return n
		}
	}
	return 8
}()

func parseRepoName(repoName string) (namespace, name string) {
	parts := strings.SplitN(repoName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "library", repoName
}

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

type ScrapeTask struct {
	URL    string
	Client *http.Client
	Token  string
}

type ParallelCrawler struct {
	WorkerCount int
	KeywordChan chan string
	ScrapeChan  chan ScrapeTask
	RepoChan    chan *myutils.Repository
	WG          sync.WaitGroup
	IM          *IdentityManager
	crawledKeys sync.Map
	pending     int32
	kwWriteWG   sync.WaitGroup
}

func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	pc := &ParallelCrawler{
		WorkerCount: workers,
		KeywordChan: make(chan string, 1000000),
		ScrapeChan:  make(chan ScrapeTask, 1000000),
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
	}
	pc.loadCrawledKeywords()
	return pc
}

func (pc *ParallelCrawler) loadCrawledKeywords() {
	if !myutils.GlobalDBClient.MongoFlag {
		return
	}
	myutils.Logger.Info("Warming up keyword cache from MongoDB (ID projection)...")
	ctx := context.Background()
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
		if len(buffer) == 0 { return }
		if err := myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer); err != nil {
			myutils.Logger.Error(fmt.Sprintf("Bulk write failed: %v", err))
		} else {
			myutils.Logger.Info(fmt.Sprintf("Flushed %d repositories to MongoDB", len(buffer)))
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case repo, ok := <-pc.RepoChan:
			if !ok { flush(); return }
			buffer = append(buffer, repo)
			if len(buffer) >= 1000 { flush() }
		case <-ticker.C:
			flush()
		}
	}
}

func (pc *ParallelCrawler) Start(seeds []string) {
	myutils.Logger.Info(fmt.Sprintf("Starting Discovery Pipeline [%d workers]", pc.WorkerCount))
	
	writerDone := make(chan struct{})
	go pc.repoWriter(context.Background(), writerDone)

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker(i)
	}

	if len(seeds) == 0 {
		for _, ch := range alphabet { seeds = append(seeds, string(ch)) }
	}
	for _, s := range seeds {
		atomic.AddInt32(&pc.pending, 1)
		pc.KeywordChan <- s
	}

	go func() {
		for {
			p := atomic.LoadInt32(&pc.pending)
			if p == 0 {
				time.Sleep(5 * time.Second)
				if atomic.LoadInt32(&pc.pending) == 0 { break }
			}
			myutils.Logger.Info(fmt.Sprintf("Discovery Progress: %d tasks pending...", p))
			time.Sleep(5 * time.Second)
		}
		close(pc.KeywordChan)
		close(pc.ScrapeChan)
	}()

	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	pc.kwWriteWG.Wait()

	if myutils.GlobalDBClient.MongoFlag {
		_ = myutils.GlobalDBClient.Mongo.DropKeywordCheckpoint()
	}
	myutils.Logger.Info("Discovery Phase Complete")
}

func (pc *ParallelCrawler) worker(id int) {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()
	if pc.WorkerCount == len(pc.IM.Accounts) {
		acc := pc.IM.Accounts[id]
		if acc.Token == "" { pc.IM.LoginDockerHub(acc) }
		token = acc.Token
		client = &http.Client{Timeout: 30 * time.Second}
	}

	for {
		select {
		case keyword, ok := <-pc.KeywordChan:
			if !ok { return }
			client, token = pc.crawlKeyword(keyword, client, token)
			atomic.AddInt32(&pc.pending, -1)
		case task, ok := <-pc.ScrapeChan:
			if !ok { return }
			pc.processPage(task.URL, task.Client, task.Token)
			atomic.AddInt32(&pc.pending, -1)
		default:
			if atomic.LoadInt32(&pc.pending) == 0 { return }
			time.Sleep(100 * time.Millisecond)
		}
	}
}

type V2Repository struct {
	RepoName  string `json:"repo_name"`
	PullCount int64  `json:"pull_count"`
}

type V2SearchResponse struct {
	Count        int            `json:"count"`
	Repositories []V2Repository `json:"results"`
}

func (pc *ParallelCrawler) crawlKeyword(keyword string, client *http.Client, token string) (*http.Client, string) {
	if _, crawled := pc.crawledKeys.Load(keyword); crawled { return client, token }

	url := myutils.GetV2SearchURL(keyword, 1, 100)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" { req.Header.Add("Authorization", "JWT "+token) }

	resp, err := client.Do(req)
	if err != nil { return client, token }
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		atomic.AddInt32(&pc.pending, 1)
		pc.KeywordChan <- keyword
		return pc.IM.GetNextClient()
	}

	var res V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return client, token }

	// Deepen Directly strategy
	if (res.Count >= 10000 || len(keyword) == 1) && len(keyword) < 255 {
		pc.saveRepos(res.Repositories)
		for _, char := range alphabet {
			atomic.AddInt32(&pc.pending, 1)
			pc.KeywordChan <- keyword + string(char)
		}
	} else if res.Count > 0 {
		totalPages := (res.Count / 100) + 1
		if totalPages > 100 { totalPages = 100 }
		for p := 1; p <= totalPages; p++ {
			atomic.AddInt32(&pc.pending, 1)
			pc.ScrapeChan <- ScrapeTask{URL: myutils.GetV2SearchURL(keyword, p, 100), Client: client, Token: token}
		}
	}
	pc.markKeywordDone(keyword)
	return client, token
}

func (pc *ParallelCrawler) markKeywordDone(keyword string) {
	pc.crawledKeys.Store(keyword, true)
	if myutils.GlobalDBClient.MongoFlag {
		pc.kwWriteWG.Add(1)
		go func() {
			defer pc.kwWriteWG.Done()
			myutils.GlobalDBClient.Mongo.MarkKeywordCrawled(keyword)
		}()
	}
}

func (pc *ParallelCrawler) processPage(url string, client *http.Client, token string) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" { req.Header.Add("Authorization", "JWT "+token) }

	resp, err := client.Do(req)
	if err != nil { return }
	defer resp.Body.Close()

	var res V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return }
	pc.saveRepos(res.Repositories)
}

func (pc *ParallelCrawler) saveRepos(v2repos []V2Repository) {
	for _, v2repo := range v2repos {
		if v2repo.RepoName == "" { continue }
		ns, name := parseRepoName(v2repo.RepoName)
		pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: v2repo.PullCount}
	}
}

func ShardSeeds(shard, total int) []string {
	chars := []rune(alphabet)
	n := len(chars)
	size := n / total
	start, end := shard*size, (shard+1)*size
	if shard == total-1 { end = n }
	var seeds []string
	for _, ch := range chars[start:end] { seeds = append(seeds, string(ch)) }
	return seeds
}
