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
	"go.mongodb.org/mongo-driver/mongo/options"
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
	// Only fetch _id to reduce network and memory overhead
	opts := options.Find().SetProjection(bson.M{"_id": 1})
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
	buffer := make([]*myutils.Repository, 0, 5000)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
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
			if !ok {
				flush()
				return
			}
			buffer = append(buffer, repo)
			if len(buffer) >= 5000 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// ShardSeeds divides the alphabet into `total` equal parts and returns the
// seed characters assigned to shard index `shard` (0-based).
//
// Example with total=2:
//
//	shard 0 → "abcdefghijklmnopqrs"   (first 19 of 38)
//	shard 1 → "tuvwxyz0123456789-_"   (last 19 of 38)
//
// This is the meet-in-the-middle partitioning: each machine independently
// explores a disjoint half of the DFS keyword tree. No coordination is needed
// because root seeds never overlap — the only shared resource is MongoDB
// (which uses upsert, so concurrent writes are safe).
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
//
// seeds specifies the root keywords to enqueue. Rules:
//   - len(seeds) > 0 → use exactly those seeds (supports --seed and --shard)
//   - len(seeds) == 0 → seed the full alphabet (backward-compatible default)
func (pc *ParallelCrawler) Start(seeds []string) {
	myutils.Logger.Info(fmt.Sprintf("Starting Parallel Crawler with %d workers", pc.WorkerCount))

	// Start background writer
	go pc.repoWriter()

	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker()
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

// worker pulls keywords from the channel and processes them. The identity
// (HTTP client + JWT token) is kept as local state and rotated whenever a 429
// is encountered, so no worker ever sleeps waiting for its rate limit to reset.
func (pc *ParallelCrawler) worker() {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()
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
		// Rotate identity immediately — no sleep. Re-enqueue the keyword so it
		// is retried by the next available worker (possibly with a different account).
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

	if searchRes.Count >= 10000 && len(keyword) < 255 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] has >= 10000 results. Deepening DFS to ensure full coverage...", keyword))
		for _, char := range alphabet {
			pc.KeywordChan <- keyword + string(char)
		}
	} else if searchRes.Count > 0 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] found %d repositories. Scraping...", keyword, searchRes.Count))
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

// processPage fetches one results page. On 429 it rotates the identity once
// and retries — avoiding the old 10 s sleep that stalled all pages in a batch.
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
		// Rotate identity and retry once before giving up on this page.
		resp.Body.Close()
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
		// Replace resp with the successful retry response
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

