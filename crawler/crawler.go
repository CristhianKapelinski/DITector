package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"crypto/tls"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var pageConcurrency = func() int {
	if v := os.Getenv("PAGE_CONCURRENCY"); v != "" {
		var n int
		fmt.Sscan(v, &n)
		if n > 0 { return n }
	}
	return 0
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
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 { return }
		count := len(buffer)
		err := myutils.GlobalDBClient.Mongo.BulkUpsertRepositories(buffer)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("DB ERROR during flush: %v", err))
			return
		}
		myutils.Logger.Info(fmt.Sprintf(">>> DATABASE UPDATE: Flushed %d NEW unique repos", count))
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
	myutils.Logger.Info(fmt.Sprintf("Starting Discovery Pipeline [W:%d]", pc.WorkerCount))
	
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

	go func() {
		for {
			p := atomic.LoadInt32(&pc.pending)
			active, _ := myutils.GlobalDBClient.Mongo.KeywordsColl.CountDocuments(context.TODO(), bson.M{"status": "pending"})
			myutils.Logger.Info(fmt.Sprintf("Status: %d workers active | %d tasks in queue", p, active))
			time.Sleep(15 * time.Second)
		}
	}()

	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	myutils.Logger.Info("Discovery Phase Complete")
}

func (pc *ParallelCrawler) ensureQueueInitialized(seeds []string) {
	if !myutils.GlobalDBClient.MongoFlag { return }
	coll := myutils.GlobalDBClient.Mongo.KeywordsColl
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	_, _ = coll.UpdateMany(ctx, bson.M{"status": "processing"}, bson.M{"$set": bson.M{"status": "pending"}})
	count, _ := coll.CountDocuments(ctx, bson.M{})
	if count > 0 { return }

	var models []mongo.WriteModel
	for _, s := range seeds {
		models = append(models, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": s}).SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending"}}).SetUpsert(true))
	}
	_, _ = coll.BulkWrite(context.TODO(), models)
}

func (pc *ParallelCrawler) worker(id int) {
	defer pc.WG.Done()
	client, token := pc.IM.GetNextClient()

	for {
		prefix := pc.getNextTask()
		if prefix == "" { break }
		
		atomic.AddInt32(&pc.pending, 1)
		success, nextClient, nextToken := pc.processTask(prefix, client, token)
		client, token = nextClient, nextToken
		atomic.AddInt32(&pc.pending, -1)

		if !success {
			time.Sleep(5 * time.Second)
		}
	}
}

func (pc *ParallelCrawler) getNextTask() string {
	if !myutils.GlobalDBClient.MongoFlag { return "" }
	var doc struct{ ID string `bson:"_id"` }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := myutils.GlobalDBClient.Mongo.KeywordsColl.FindOneAndUpdate(
		ctx,
		bson.M{"status": "pending"},
		bson.M{"$set": bson.M{"status": "processing", "started_at": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetSort(bson.M{"finished_at": 1}),
	).Decode(&doc)
	if err != nil { return "" }
	return doc.ID
}

func (pc *ParallelCrawler) processTask(prefix string, client *http.Client, token string) (bool, *http.Client, string) {
	myutils.Logger.Info(fmt.Sprintf("Processing Prefix: [%s]", prefix))
	
	res, nextClient, nextToken := pc.fetchPage(prefix, 1, client, token)
	client, token = nextClient, nextToken
	if res == nil {
		pc.updateTaskStatus(prefix, "pending")
		return false, client, token
	}

	pc.processResults(res.Repositories)
	pages := (res.Count / 100) + 1
	if pages > 100 { pages = 100 }
	for p := 2; p <= pages; p++ {
		time.Sleep(time.Duration(400 + rand.Intn(500)) * time.Millisecond)
		resP, c, t := pc.fetchPage(prefix, p, client, token)
		client, token = c, t
		if resP != nil { 
			pc.processResults(resP.Repositories)
		} else {
			pc.updateTaskStatus(prefix, "pending")
			return false, client, token
		}
	}

	if (res.Count >= 10000 || len(prefix) == 1) && len(prefix) < 255 {
		var models []mongo.WriteModel
		for _, char := range alphabet {
			models = append(models, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": prefix + string(char)}).SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending"}}).SetUpsert(true))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.BulkWrite(ctx, models)
	}
	
	pc.updateTaskStatus(prefix, "done")
	return true, client, token
}

func (pc *ParallelCrawler) updateTaskStatus(id, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status, "finished_at": time.Now()}})
}

func (pc *ParallelCrawler) fetchPage(query string, page int, client *http.Client, token string) (*V2SearchResponse, *http.Client, string) {
	for attempts := 0; attempts < 3; attempts++ {
		url := myutils.GetV2SearchURL(query, page, 100)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		if token != "" { req.Header.Add("Authorization", "JWT "+token) }

		resp, err := client.Do(req)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("Network Error for %q: %v. Refreshing connection...", query, err))
			newC, newT := pc.IM.GetNextClient()
			return nil, newC, newT
		}
		defer resp.Body.Close()

		// 401 Handle: Expired or Revoked Token. Triggering Centralized Re-login.
		if resp.StatusCode == 401 {
			myutils.Logger.Warn(fmt.Sprintf("401 Unauthorized for %q. Invalidating token and rotating...", query))
			pc.IM.ClearToken(token)
			newC, newT := pc.IM.GetNextClient()
			return nil, newC, newT
		}

		if resp.StatusCode == 429 {
			myutils.Logger.Warn(fmt.Sprintf("429 Rate Limit for %q. Rotating identity...", query))
			time.Sleep(15 * time.Second)
			newC, newT := pc.IM.GetNextClient()
			return nil, newC, newT
		}

		if resp.StatusCode != http.StatusOK { 
			myutils.Logger.Error(fmt.Sprintf("Unexpected Status %d for %q", resp.StatusCode, query))
			return nil, client, token 
		}
		var res V2SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return nil, client, token }
		return &res, client, token
	}
	return nil, client, token
}

func (pc *ParallelCrawler) processResults(repos []struct {
	RepoName  string "json:\"repo_name\""
	PullCount int64  "json:\"pull_count\""
}) {
	for _, r := range repos {
		if r.RepoName == "" { continue }
		if _, seen := pc.seenRepos.LoadOrStore(r.RepoName, true); seen { continue }
		ns, name := parseRepoName(r.RepoName)
		select {
		case pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: r.PullCount}:
		default:
			myutils.Logger.Warn("RepoChan FULL! Data may be delayed.")
		}
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
