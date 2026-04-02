package crawler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
)

// Alphabet for DFS keyword generation
const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_"

// ParallelCrawler handles the distributed crawling logic
type ParallelCrawler struct {
	WorkerCount int
	KeywordChan chan string
	WG          sync.WaitGroup
	IM          *IdentityManager
}

// NewParallelCrawler initializes a new crawler
func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	return &ParallelCrawler{
		WorkerCount: workers,
		KeywordChan: make(chan string, 1000000), // Buffer for DFS keywords
		IM:          im,
	}
}

// Start initiates the parallel crawl
func (pc *ParallelCrawler) Start(seed string) {
	myutils.Logger.Info(fmt.Sprintf("Starting Parallel Crawler with %d workers", pc.WorkerCount))

	// Launch workers
	for i := 0; i < pc.WorkerCount; i++ {
		pc.WG.Add(1)
		go pc.worker()
	}

	// Initial seed keywords
	if seed != "" {
		myutils.Logger.Info(fmt.Sprintf("Seeding crawler with: %s", seed))
		pc.KeywordChan <- seed
	} else {
		for _, char := range alphabet {
			pc.KeywordChan <- string(char)
		}
	}

	// Wait for workers to finish
	pc.WG.Wait()
}

func (pc *ParallelCrawler) worker() {
	defer pc.WG.Done()
	// Each worker gets its own identity rotation client
	client, token := pc.IM.GetNextClient()
	for keyword := range pc.KeywordChan {
		pc.crawlKeyword(keyword, client, token)
	}
}

type V2Repository struct {
	RepoName  string `json:"repo_name"`
	RepoOwner string `json:"repo_owner"`
	PullCount int64  `json:"pull_count"`
}

type V2SearchResponse struct {
	Count      int            `json:"count"`
	Repositories []V2Repository `json:"results"`
}

func (pc *ParallelCrawler) crawlKeyword(keyword string, client *http.Client, token string) {
	// Add a small delay between keywords to avoid 429
	time.Sleep(500 * time.Millisecond)

	url := myutils.GetV2SearchURL(keyword, 1, 100)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if token != "" {
		req.Header.Add("Authorization", "JWT "+token)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("Request failed for keyword [%s]: %v", keyword, err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		myutils.Logger.Warn(fmt.Sprintf("Keyword [%s] got 429. Sleeping 30s...", keyword))
		time.Sleep(30 * time.Second)
		return
	}

	if resp.StatusCode != http.StatusOK {
		myutils.Logger.Warn(fmt.Sprintf("Keyword [%s] got status %d", keyword, resp.StatusCode))
		return
	}

	var searchRes V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		myutils.Logger.Error(fmt.Sprintf("JSON decode failed for keyword [%s]: %v", keyword, err))
		return
	}

	// 2. DFS Strategy
	if searchRes.Count >= 10000 && len(keyword) < 5 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] has %d results. Deepening DFS...", keyword, searchRes.Count))
		for _, char := range alphabet {
			pc.KeywordChan <- keyword + string(char)
		}
	} else if searchRes.Count > 0 {
		myutils.Logger.Info(fmt.Sprintf("Keyword [%s] found %d repositories. Scraping...", keyword, searchRes.Count))
		pc.scrapeAllPages(keyword, searchRes.Count, client, token)
	} else {
		myutils.Logger.Warn(fmt.Sprintf("Keyword [%s] returned 0 results.", keyword))
	}
}

func (pc *ParallelCrawler) scrapeAllPages(keyword string, totalCount int, client *http.Client, token string) {
	totalPages := (totalCount / 100) + 1
	if totalPages > 100 { totalPages = 100 }

	for page := 1; page <= totalPages; page++ {
		url := myutils.GetV2SearchURL(keyword, page, 100)
		pc.processPage(url, client, token)
		time.Sleep(200 * time.Millisecond) // Pagination delay
	}
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
		time.Sleep(10 * time.Second)
		return
	}

	var searchRes V2SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil {
		return
	}

	for _, v2repo := range searchRes.Repositories {
		repo := &myutils.Repository{
			Namespace: v2repo.RepoOwner,
			Name:      v2repo.RepoName,
			PullCount: v2repo.PullCount,
		}
		if repo.Name == "" { continue }

		myutils.Logger.Info(fmt.Sprintf("Discovered repository: %s/%s (%d pulls)", repo.Namespace, repo.Name, repo.PullCount))
		if myutils.GlobalDBClient.MongoFlag {
			err := myutils.GlobalDBClient.Mongo.UpdateRepository(repo)
			if err != nil {
				myutils.Logger.Error(fmt.Sprintf("Failed to update repo %s/%s: %v", repo.Namespace, repo.Name, err))
			}
		}
	}
}
