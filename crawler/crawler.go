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
		if n > 0 { return n }
	}
	return 8
}()

func parseRepoName(repoName string) (namespace, name string) {
	parts := strings.SplitN(repoName, "/", 2)
	if len(parts) == 2 { return parts[0], parts[1] }
	return "library", repoName
}

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

type ScrapeTask struct {
	URL    string
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
	return &ParallelCrawler{
		WorkerCount: workers,
		KeywordChan: make(chan string, 1000000),
		ScrapeChan:  make(chan ScrapeTask, 1000000),
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
	}
}

func (pc *ParallelCrawler) loadCrawledKeywords() {
	if !myutils.GlobalDBClient.MongoFlag { return }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	opts := mongodb_opts.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := myutils.GlobalDBClient.Mongo.KeywordsColl.Find(ctx, bson.M{}, opts)
	if err != nil { return }
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
		_ = myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer)
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
	pc.loadCrawledKeywords()
	myutils.Logger.Info(fmt.Sprintf("Starting Production Pipeline [W:%d]", pc.WorkerCount))
	
	writerDone := make(chan struct{})
	go pc.repoWriter(context.Background(), writerDone)

	for i := 0; i < 2; i++ {
		pc.WG.Add(1)
		go pc.discoveryWorker()
	}

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.scrapeWorker(i)
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
			myutils.Logger.Info(fmt.Sprintf("Discovery Progress: %d tasks active", p))
			time.Sleep(10 * time.Second)
		}
		close(pc.KeywordChan)
		close(pc.ScrapeChan)
	}()

	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	pc.kwWriteWG.Wait()
	myutils.Logger.Info("Full Cycle Complete")
}

func (pc *ParallelCrawler) discoveryWorker() {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()
	for keyword := range pc.KeywordChan {
		client, token = pc.discover(keyword, client, token)
		atomic.AddInt32(&pc.pending, -1)
	}
}

func (pc *ParallelCrawler) scrapeWorker(id int) {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()
	if pc.WorkerCount == len(pc.IM.Accounts) {
		acc := pc.IM.Accounts[id]
		if acc.Token == "" { pc.IM.LoginDockerHub(acc) }
		token = acc.Token
		client = &http.Client{Timeout: 30 * time.Second}
	}

	for task := range pc.ScrapeChan {
		pc.processPage(task.URL, client, token)
		atomic.AddInt32(&pc.pending, -1)
	}
}

type V2SearchResponse struct {
	Count        int            `json:"count"`
	Repositories []struct {
		RepoName  string `json:"repo_name"`
		PullCount int64  `json:"pull_count"`
	} `json:"results"`
}

func (pc *ParallelCrawler) discover(keyword string, client *http.Client, token string) (*http.Client, string) {
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

	// SCRAPE FIRST: Always scrape available results to ensure immediate database growth
	if res.Count > 0 {
		pages := (res.Count / 100) + 1
		if pages > 100 { pages = 100 }
		for p := 1; p <= pages; p++ {
			atomic.AddInt32(&pc.pending, 1)
			pc.ScrapeChan <- ScrapeTask{URL: myutils.GetV2SearchURL(keyword, p, 100)}
		}
	}

	// THEN DEEPEN: If it hits the 10k limit, also fan out
	if (res.Count >= 10000 || len(keyword) == 1) && len(keyword) < 255 {
		for _, char := range alphabet {
			atomic.AddInt32(&pc.pending, 1)
			pc.KeywordChan <- keyword + string(char)
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
			_ = myutils.GlobalDBClient.Mongo.MarkKeywordCrawled(keyword)
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
	for _, r := range res.Repositories {
		if r.RepoName == "" { continue }
		ns, name := parseRepoName(r.RepoName)
		pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: r.PullCount}
	}
}
