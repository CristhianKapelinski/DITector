package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
	mongodb_opts "go.mongodb.org/mongo-driver/mongo/options"
)

// pageConcurrency controls how many pages of a single keyword are fetched in
// parallel. Override with env var PAGE_CONCURRENCY (e.g. PAGE_CONCURRENCY=16).
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

// parseRepoName splits "namespace/name" from the V2 API repo_name field.
// Official images (e.g. "nginx") are treated as "library/nginx".
func parseRepoName(repoName string) (namespace, name string) {
	parts := strings.SplitN(repoName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "library", repoName
}

// Alphabet for DFS keyword generation
const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

// ParallelCrawler handles the distributed crawling logic
type ParallelCrawler struct {
	WorkerCount int
	KeywordChan chan string
	RepoChan    chan *myutils.Repository
	WG          sync.WaitGroup
	IM          *IdentityManager
	crawledKeys sync.Map // cache for already crawled keywords
}

// NewParallelCrawler initializes a new crawler
func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	pc := &ParallelCrawler{
		WorkerCount: workers,
		KeywordChan: make(chan string, 1000000),
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
	}
	pc.loadCrawledKeywords()
	return pc
}

// loadCrawledKeywords warms up the in-memory cache from MongoDB.
func (pc *ParallelCrawler) loadCrawledKeywords() {
	if !myutils.GlobalDBClient.MongoFlag {
		return
	}
	myutils.Logger.Info("Warming up keyword cache from MongoDB (ID projection)...")
	ctx := context.Background()
	opts := mongodb_opts.Find().SetProjection(bson.M{"_id": 1})
	cursor, err := myutils.GlobalDBClient.Mongo.KeywordsColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("Failed to load keyword cache: %v", err))
		return
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err == nil {
			pc.crawledKeys.Store(doc.ID, true)
			count++
		}
	}
	myutils.Logger.Info(fmt.Sprintf("Cache ready: %d keywords loaded", count))
}

// repoWriter aggregates repositories and performs large bulk writes.
func (pc *ParallelCrawler) repoWriter() {
	buffer := make([]*myutils.Repository, 0, 1000)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if err := myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer); err != nil {
			myutils.Logger.Error(fmt.Sprintf("Bulk write failed: %v", err))
		} else {
			myutils.Logger.Info(fmt.Sprintf("Flushed %d repositories to MongoDB (Rate monitoring active)", len(buffer)))
		}
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

// ShardSeeds divides the alphabet into `total` equal parts and returns the
// seed characters assigned to shard index `shard` (0-based).
func ShardSeeds(shard, total int) []string {
	chars := []rune(alphabet)
	n := len(chars)
	size := n / total
	start := shard * size
	end := start + size
	if shard == total-1 {
		end = n // last shard takes any remainder
	}
	seeds := make([]string, end-start)
	for i, ch := range chars[start:end] {
		seeds[i] = string(ch)
	}
	return seeds
}

// Start initiates the parallel crawl.
func (pc *ParallelCrawler) Start(seeds []string) {
	myutils.Logger.Info(fmt.Sprintf("Starting Parallel Crawler with %d workers", pc.WorkerCount))

	// Start background writer
	go pc.repoWriter()

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker(i)
	}

	if len(seeds) > 0 {
		myutils.Logger.Info(fmt.Sprintf("Seeding crawler with %d root keywords: %v", len(seeds), seeds))
		for _, s := range seeds {
			pc.KeywordChan <- s
		}
	} else {
		myutils.Logger.Info("Seeding crawler with full alphabet (38 root keywords)")
		for _, ch := range alphabet {
			pc.KeywordChan <- string(ch)
		}
	}

	pc.WG.Wait()
	close(pc.RepoChan)
}

// worker pulls keywords from the channel and processes them.
func (pc *ParallelCrawler) worker(workerID int) {
	defer pc.WG.Done()

	var client *http.Client
	var token string

	// Strict Account Isolation: if workers == accounts, lock each worker to one account
	if pc.WorkerCount == len(pc.IM.Accounts) {
		acc := pc.IM.Accounts[workerID]
		if acc.Token == "" {
			pc.IM.LoginDockerHub(acc)
		}
		token = acc.Token
		// Dedicated transport for this worker to avoid connection pooling issues between accounts
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxConnsPerHost:     10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
		myutils.Logger.Info(fmt.Sprintf("Worker %d strictly isolated to account: %s", workerID, acc.Username))
	} else {
		client, token = pc.IM.GetNextClient()
	}

	for keyword := range pc.KeywordChan {
		client, token = pc.crawlKeyword(keyword, client, token)
	}
}

type V2Repository struct {
	RepoName  string `json:"repo_name"`
	RepoOwner string `json:"repo_owner"`
	PullCount int64  `json:"pull_count"`
}

type V2SearchResponse struct {
	Count        int            `json:"count"`
	Repositories []V2Repository `json:"results"`
}

// crawlKeyword processes one keyword and returns the (possibly rotated)
// identity to use for the next keyword. On 429 the keyword is re-enqueued
// and a fresh identity is returned immediately — no sleep.
func (pc *ParallelCrawler) crawlKeyword(keyword string, client *http.Client, token string) (*http.Client, string) {
	if _, crawled := pc.crawledKeys.Load(keyword); crawled {
		myutils.Logger.Debug(fmt.Sprintf("Keyword [%s] already crawled (cache), skipping", keyword))
		return client, token
	}

	url := myutils.GetV2SearchURL(keyword, 1, 100)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" {
		req.Header.Add("Authorization", "JWT "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("Request failed for keyword [%s]: %v", keyword, err))
		return client, token
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		myutils.Logger.Warn(fmt.Sprintf("Keyword [%s] got 429. Rotating identity...", keyword))
		pc.KeywordChan <- keyword
		return pc.IM.GetNextClient()
	}

	if resp.StatusCode != http.StatusOK {
		myutils.Logger.Warn(fmt.Sprintf("Keyword [%s] got status %d", keyword, resp.StatusCode))
		return client, token
	}

	var searchRes V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		myutils.Logger.Error(fmt.Sprintf("JSON decode failed for keyword [%s]: %v", keyword, err))
		return client, token
	}

	// Deepen Directly strategy: skip scraping parent nodes with 10k results
	if searchRes.Count >= 10000 && len(keyword) < 255 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] has >= 10000 results (%d). Deepening DFS directly...", keyword, searchRes.Count))
		for _, char := range alphabet {
			pc.KeywordChan <- keyword + string(char)
		}
		return client, token
	}

	if searchRes.Count > 0 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] found %d repositories. Scraping all pages...", keyword, searchRes.Count))
		pc.scrapeAllPages(keyword, searchRes.Count, client, token)
		pc.crawledKeys.Store(keyword, true)
		if myutils.GlobalDBClient.MongoFlag {
			go func(k string) {
				if err := myutils.GlobalDBClient.Mongo.MarkKeywordCrawled(k); err != nil {
					myutils.Logger.Error(fmt.Sprintf("MarkKeywordCrawled [%s] failed: %v", k, err))
				}
			}(keyword)
		}
	} else {
		myutils.Logger.Debug(fmt.Sprintf("Keyword [%s] returned 0 results.", keyword))
		pc.crawledKeys.Store(keyword, true)
		if myutils.GlobalDBClient.MongoFlag {
			go myutils.GlobalDBClient.Mongo.MarkKeywordCrawled(keyword)
		}
	}
	return client, token
}

func (pc *ParallelCrawler) scrapeAllPages(keyword string, totalCount int, client *http.Client, token string) {
	totalPages := (totalCount / 100) + 1
	if totalPages > 100 {
		totalPages = 100
	}

	sem := make(chan struct{}, pageConcurrency)
	var wg sync.WaitGroup
	for page := 1; page <= totalPages; page++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()
			pc.processPage(myutils.GetV2SearchURL(keyword, p, 100), client, token)
		}(page)
	}
	wg.Wait()
}

func (pc *ParallelCrawler) processPage(url string, client *http.Client, token string) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" {
		req.Header.Add("Authorization", "JWT "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("Page request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		newClient, newToken := pc.IM.GetNextClient()
		req2, _ := http.NewRequest("GET", url, nil)
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		if newToken != "" {
			req2.Header.Add("Authorization", "JWT "+newToken)
		}
		resp2, err2 := newClient.Do(req2)
		if err2 != nil || resp2.StatusCode != http.StatusOK {
			if resp2 != nil {
				resp2.Body.Close()
			}
			return
		}
		defer resp2.Body.Close()
		var searchRes V2SearchResponse
		if err := json.NewDecoder(resp2.Body).Decode(&searchRes); err != nil {
			return
		}
		pc.saveRepos(searchRes.Repositories)
		return
	}

	var searchRes V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return
	}
	pc.saveRepos(searchRes.Repositories)
}

func (pc *ParallelCrawler) saveRepos(v2repos []V2Repository) {
	for _, v2repo := range v2repos {
		if v2repo.RepoName == "" {
			continue
		}
		ns, name := parseRepoName(v2repo.RepoName)
		pc.RepoChan <- &myutils.Repository{
			Namespace: ns,
			Name:      name,
			PullCount: v2repo.PullCount,
		}
	}
}
