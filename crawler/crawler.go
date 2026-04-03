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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var pageConcurrency = func() int {
	if v := os.Getenv("PAGE_CONCURRENCY"); v != "" {
		var n int
		fmt.Sscan(v, &n)
		if n > 0 { return n }
	}
	return 0 // Default to serial (Python-mimic) if not specified
}()

func parseRepoName(repoName string) (namespace, name string) {
	parts := strings.SplitN(repoName, "/", 2)
	if len(parts) == 2 { return parts[0], parts[1] }
	return "library", repoName
}

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

type ParallelCrawler struct {
	WorkerCount int
	RepoChan    chan *myutils.Repository
	WG          sync.WaitGroup
	IM          *IdentityManager
	seenRepos   sync.Map
	pending     int32
}

func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	return &ParallelCrawler{
		WorkerCount: workers,
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
	}
}

func (pc *ParallelCrawler) repoWriter(ctx context.Context, done chan struct{}) {
	defer close(done)
	buffer := make([]*myutils.Repository, 0, 1000)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 { return }
		_ = myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer)
		myutils.Logger.Info(fmt.Sprintf("Flushed %d unique repos", len(buffer)))
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
	myutils.Logger.Info(fmt.Sprintf("Starting Discovery Pipeline [W:%d, PC:%d]", pc.WorkerCount, pageConcurrency))
	
	// Initialize task queue in MongoDB
	if len(seeds) == 0 {
		for _, ch := range alphabet { seeds = append(seeds, string(ch)) }
	}
	pc.ensureQueueInitialized(seeds)

	writerDone := make(chan struct{})
	go pc.repoWriter(context.Background(), writerDone)

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker(i)
	}

	// Liveness Monitor
	go func() {
		for {
			p := atomic.LoadInt32(&pc.pending)
			active, _ := myutils.GlobalDBClient.Mongo.KeywordsColl.CountDocuments(context.TODO(), bson.M{"status": "pending"})
			if p == 0 && active == 0 {
				time.Sleep(5 * time.Second)
				active, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.CountDocuments(context.TODO(), bson.M{"status": "pending"})
				if active == 0 { break }
			}
			myutils.Logger.Info(fmt.Sprintf("Progress: %d local workers active | %d tasks in queue", p, active))
			time.Sleep(10 * time.Second)
		}
		// No need to close a channel, workers will exit when getNextTask returns empty
	}()

	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	myutils.Logger.Info("Discovery Phase Complete")
}

func (pc *ParallelCrawler) ensureQueueInitialized(seeds []string) {
	if !myutils.GlobalDBClient.MongoFlag { return }
	coll := myutils.GlobalDBClient.Mongo.KeywordsColl

	// Self-healing: reset stuck 'processing' tasks back to 'pending'
	_, _ = coll.UpdateMany(context.TODO(), 
		bson.M{"status": "processing"}, 
		bson.M{"$set": bson.M{"status": "pending"}},
	)

	count, _ := coll.CountDocuments(context.TODO(), bson.M{})
	if count > 0 { return }

	myutils.Logger.Info("Initializing task queue with root seeds...")
	var models []mongo.WriteModel
	for _, s := range seeds {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": s}).
			SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending"}}).
			SetUpsert(true))
	}
	_, _ = coll.BulkWrite(context.TODO(), models)
}

func (pc *ParallelCrawler) worker(id int) {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()
	
	// Strict Account Isolation
	if pc.WorkerCount == len(pc.IM.Accounts) {
		acc := pc.IM.Accounts[id]
		if acc.Token == "" { pc.IM.LoginDockerHub(acc) }
		token = acc.Token
		client = &http.Client{Timeout: 30 * time.Second}
	}

	for {
		prefix := pc.getNextTask()
		if prefix == "" { break }

		atomic.AddInt32(&pc.pending, 1)
		client, token = pc.processTask(prefix, client, token)
		atomic.AddInt32(&pc.pending, -1)
	}
}

func (pc *ParallelCrawler) getNextTask() string {
	if !myutils.GlobalDBClient.MongoFlag { return "" }
	coll := myutils.GlobalDBClient.Mongo.KeywordsColl
	var doc struct{ ID string `bson:"_id"` }
	
	err := coll.FindOneAndUpdate(
		context.TODO(),
		bson.M{"status": "pending"},
		bson.M{"$set": bson.M{"status": "processing", "started_at": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)

	if err != nil { return "" }
	return doc.ID
}

func (pc *ParallelCrawler) processTask(prefix string, client *http.Client, token string) (*http.Client, string) {
	res, nextClient, nextToken := pc.fetchPage(prefix, 1, client, token)
	client, token = nextClient, nextToken
	if res == nil {
		pc.updateTaskStatus(prefix, "pending")
		return client, token
	}

	// Scrape
	pc.processResults(res.Repositories)
	if pageConcurrency > 0 {
		pc.scrapeAllPages(prefix, res.Count, client, token)
	} else {
		// Serial fallback
		pages := (res.Count / 100) + 1
		if pages > 100 { pages = 100 }
		for p := 2; p <= pages; p++ {
			time.Sleep(200 * time.Millisecond)
			resP, c, t := pc.fetchPage(prefix, p, client, token)
			client, token = c, t
			if resP != nil { pc.processResults(resP.Repositories) }
		}
	}

	// Fan-out
	if (res.Count >= 10000 || len(prefix) == 1) && len(prefix) < 255 {
		var models []mongo.WriteModel
		for _, char := range alphabet {
			models = append(models, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": prefix + string(char)}).
				SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending"}}).
				SetUpsert(true))
		}
		_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.BulkWrite(context.TODO(), models)
	}

	pc.updateTaskStatus(prefix, "done")
	return client, token
}

func (pc *ParallelCrawler) updateTaskStatus(id, status string) {
	_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.UpdateOne(
		context.TODO(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": status, "finished_at": time.Now()}},
	)
}

func (pc *ParallelCrawler) scrapeAllPages(keyword string, totalCount int, client *http.Client, token string) {
	totalPages := (totalCount / 100) + 1
	if totalPages > 100 { totalPages = 100 }
	sem := make(chan struct{}, pageConcurrency)
	var wg sync.WaitGroup
	for p := 1; p <= totalPages; p++ {
		wg.Add(1); sem <- struct{}{}
		go func(page int) {
			defer wg.Done(); defer func() { <-sem }()
			res, _, _ := pc.fetchPage(keyword, page, client, token)
			if res != nil { pc.processResults(res.Repositories) }
		}(p)
	}
	wg.Wait()
}

func (pc *ParallelCrawler) fetchPage(query string, page int, client *http.Client, token string) (*V2SearchResponse, *http.Client, string) {
	url := myutils.GetV2SearchURL(query, page, 100)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" { req.Header.Add("Authorization", "JWT "+token) }

	resp, err := client.Do(req)
	if err != nil { return nil, client, token }
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		myutils.Logger.Warn("429. Rotating...")
		time.Sleep(10 * time.Second)
		newC, newT := pc.IM.GetNextClient()
		return pc.fetchPage(query, page, newC, newT)
	}
	if resp.StatusCode != http.StatusOK { return nil, client, token }
	var res V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return nil, client, token }
	return &res, client, token
}

func (pc *ParallelCrawler) processResults(repos []struct {
	RepoName  string "json:\"repo_name\""
	PullCount int64  "json:\"pull_count\""
}) {
	for _, r := range repos {
		if r.RepoName == "" { continue }
		if _, seen := pc.seenRepos.LoadOrStore(r.RepoName, true); seen { continue }
		ns, name := parseRepoName(r.RepoName)
		pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: r.PullCount}
	}
}

type V2SearchResponse struct {
	Count        int `json:"count"`
	Repositories []struct {
		RepoName  string `json:"repo_name"`
		PullCount int64  `json:"pull_count"`
	} `json:"results"`
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
