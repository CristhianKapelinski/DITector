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
	"io"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Updated User-Agents to match the high-version found in browser dump
var uaWindows = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36",
}
var uaLinuxMac = []string{
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
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
	startTime   time.Time
}

func NewParallelCrawler(workers int, im *IdentityManager) *ParallelCrawler {
	return &ParallelCrawler{
		WorkerCount: workers,
		RepoChan:    make(chan *myutils.Repository, 100000),
		IM:          im,
		startTime:   time.Now(),
	}
}

func (pc *ParallelCrawler) PreloadExistingRepos() {
	if !myutils.GlobalDBClient.MongoFlag { return }
	myutils.Logger.Info(">>> CACHE WARM-UP: Loading existing dataset to RAM...")
	coll := myutils.GlobalDBClient.Mongo.RepoColl
	projection := bson.M{"namespace": 1, "name": 1, "_id": 0}
	cursor, err := coll.Find(context.TODO(), bson.M{}, options.Find().SetProjection(projection))
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("!!! CRITICAL: Failed to preload repos: %v", err))
		return
	}
	defer cursor.Close(context.TODO())
	count := 0
	start := time.Now()
	for cursor.Next(context.TODO()) {
		var r struct {
			Namespace string `bson:"namespace"`
			Name      string `bson:"name"`
		}
		if err := cursor.Decode(&r); err != nil { continue }
		pc.seenRepos.Store(r.Namespace+"/"+r.Name, true)
		count++
		if count % 250000 == 0 {
			myutils.Logger.Info(fmt.Sprintf("... Preloading: %.1f Million repos in RAM", float64(count)/1000000.0))
		}
	}
	myutils.Logger.Info(fmt.Sprintf(">>> WARM-UP COMPLETE: %d repos loaded in %v", count, time.Since(start).Round(time.Second)))
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
			myutils.Logger.Error(fmt.Sprintf("!!! DATABASE ERROR: %v", err))
			return
		}
		myutils.Logger.Info(fmt.Sprintf(">>> DB SYNC: Flushed %d NEW repos to central database", count))
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
	pc.PreloadExistingRepos()
	myutils.Logger.Info(fmt.Sprintf(">>> CORE START: Discovery Pipeline Active [W:%d]", pc.WorkerCount))
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
			myutils.Logger.Info(fmt.Sprintf("--- STATS: %d workers active | %d tasks left | Uptime: %v", p, active, time.Since(pc.startTime).Round(time.Second)))
			time.Sleep(30 * time.Second)
		}
	}()
	pc.WG.Wait()
	close(pc.RepoChan)
	<-writerDone
	myutils.Logger.Info(">>> PIPELINE HALTED: Discovery Cycle Finished")
}

func (pc *ParallelCrawler) ensureQueueInitialized(seeds []string) {
	if !myutils.GlobalDBClient.MongoFlag { return }
	coll := myutils.GlobalDBClient.Mongo.KeywordsColl
	_, _ = coll.UpdateMany(context.TODO(), bson.M{"status": "processing"}, bson.M{"$set": bson.M{"status": "pending"}})
	count, _ := coll.CountDocuments(context.TODO(), bson.M{})
	if count > 0 { return }
	var models []mongo.WriteModel
	for _, s := range seeds {
		models = append(models, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": s}).SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending"}}).SetUpsert(true))
	}
	_, _ = coll.BulkWrite(context.TODO(), models)
}

func (pc *ParallelCrawler) setBrowserHeaders(req *http.Request, token, ua string) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Referer", "https://hub.docker.com/search?q=")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-Ch-Ua", "\"Not:A-Brand\";v=\"99\", \"Google Chrome\";v=\"145\", \"Chromium\";v=\"145\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"Linux\"")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Connection", "keep-alive")
	if token != "" { req.Header.Set("Authorization", "JWT "+token) }
}

func (pc *ParallelCrawler) worker(id int) {
	defer pc.WG.Done()
	role := os.Getenv("ROLE")
	var myUA string
	if role == "primary" {
		myUA = uaWindows[rand.Intn(len(uaWindows))]
	} else {
		myUA = uaLinuxMac[rand.Intn(len(uaLinuxMac))]
	}
	client, token, _ := pc.IM.GetNextClient()
	for {
		prefix := pc.getNextTask()
		if prefix == "" { break }
		atomic.AddInt32(&pc.pending, 1)
		success, nextClient, nextToken, nextUA := pc.processTask(prefix, client, token, myUA)
		client, token, myUA = nextClient, nextToken, nextUA
		atomic.AddInt32(&pc.pending, -1)
		if !success { time.Sleep(5 * time.Second) }
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
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
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "_id", Value: 1}}),
	).Decode(&doc)
	if err != nil { return "" }
	return doc.ID
}

func (pc *ParallelCrawler) processTask(prefix string, client *http.Client, token, ua string) (bool, *http.Client, string, string) {
	uaPreview := ua
	if len(uaPreview) > 30 { uaPreview = uaPreview[:30] }
	myutils.Logger.Info(fmt.Sprintf("[TASK] Exploring: [%s] | Identity: %s...", prefix, uaPreview))
	res, nextClient, nextToken, nextUA := pc.fetchPage(prefix, 1, client, token, ua)
	client, token, ua = nextClient, nextToken, nextUA
	if res == nil {
		pc.updateTaskStatus(prefix, "pending")
		return false, client, token, ua
	}
	newInPrefix := pc.processResults(res.Repositories)
	pages := (res.Count / 100) + 1
	if pages > 100 { pages = 100 }
	for p := 2; p <= pages; p++ {
		time.Sleep(time.Duration(400 + rand.Intn(500)) * time.Millisecond)
		resP, c, t, u := pc.fetchPage(prefix, p, client, token, ua)
		client, token, ua = c, t, u
		if resP == nil {
			pc.updateTaskStatus(prefix, "pending")
			return false, client, token, ua
		}
		if len(resP.Repositories) == 0 { break }
		newInPrefix += pc.processResults(resP.Repositories)
	}
	if (res.Count >= 10000 || len(prefix) == 1) && len(prefix) < 255 {
		tokenPlateau := newInPrefix == 0 && res.Count >= 10000 && strings.Contains(prefix, "-") && len(prefix) > 1
		lastChar := prefix[len(prefix)-1]
		isSep := lastChar == '-' || lastChar == '_'
		var models []mongo.WriteModel
		for _, char := range alphabet {
			if isSep && (char == '-' || char == '_') { continue }
			child := prefix + string(char)
			priority := 0
			if tokenPlateau {
				priority = -1
			} else {
				if newInPrefix > 0 { priority = 1 }
				if !strings.Contains(child, "-") { priority = 2 }
			}
			models = append(models, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": child}).
				SetUpdate(bson.M{"$setOnInsert": bson.M{"status": "pending", "priority": priority}}).
				SetUpsert(true))
		}
		if tokenPlateau {
			myutils.Logger.Info(fmt.Sprintf(">>> DEPRIORITIZING [%s]: token-match plateau (%d results, 0 new). Children set to priority=-1.", prefix, res.Count))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.BulkWrite(ctx, models)
	}
	efficiency := (float64(newInPrefix) / float64(pages*100)) * 100.0
	myutils.Logger.Info(fmt.Sprintf("[DONE] Prefix [%s]: +%d unique | Eff: %.1f%% | Found total: %d", prefix, newInPrefix, efficiency, res.Count))
	pc.updateTaskStatus(prefix, "done")
	return true, client, token, ua
}

func (pc *ParallelCrawler) updateTaskStatus(id, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = myutils.GlobalDBClient.Mongo.KeywordsColl.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status, "finished_at": time.Now()}})
}

func (pc *ParallelCrawler) fetchPage(query string, page int, client *http.Client, token, ua string) (*V2SearchResponse, *http.Client, string, string) {
	for attempts := 0; attempts < 3; attempts++ {
		url := myutils.GetV2SearchURL(query, page, 100)
		req, _ := http.NewRequest("GET", url, nil)
		pc.setBrowserHeaders(req, token, ua)
		resp, err := client.Do(req)
		if err != nil {
			myutils.Logger.Error(fmt.Sprintf("!!! NET ERROR [%s]: %v. Rotating...", query, err))
			client, token, ua = pc.IM.GetNextClient()
			continue
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			var res V2SearchResponse
			if err := json.Unmarshal(bodyBytes, &res); err != nil { return nil, client, token, ua }
			return &res, client, token, ua
		case 401:
			myutils.Logger.Warn(fmt.Sprintf("!!! 401 [%s]. Rotating account...", query))
			pc.IM.ClearToken(token)
			client, token, ua = pc.IM.GetNextClient()
			continue
		case 429:
			myutils.Logger.Warn(fmt.Sprintf("!!! 429 [%s]. Rate limit, rotating...", query))
			time.Sleep(15 * time.Second)
			client, token, ua = pc.IM.GetNextClient()
			continue
		case 403:
			myutils.Logger.Error(fmt.Sprintf("!!! 403 [%s]. Bot block detected, rotating...", query))
			client, token, ua = pc.IM.GetNextClient()
			continue
		case 404:
			return &V2SearchResponse{}, client, token, ua
		default:
			body := string(bodyBytes)
			if len(body) > 200 { body = body[:200] }
			myutils.Logger.Warn(fmt.Sprintf("!!! HTTP %d [%s]. Body: %s. Cooling off 4m...", resp.StatusCode, query, body))
			time.Sleep(4 * time.Minute)
			return nil, client, token, ua
		}
	}
	return nil, client, token, ua
}

func (pc *ParallelCrawler) processResults(repos []struct {
	RepoName  string "json:\"repo_name\""
	PullCount int64  "json:\"pull_count\""
}) int {
	newCount := 0
	for _, r := range repos {
		if r.RepoName == "" { continue }
		if _, seen := pc.seenRepos.LoadOrStore(r.RepoName, true); seen { continue }
		ns, name := parseRepoName(r.RepoName)
		select {
		case pc.RepoChan <- &myutils.Repository{Namespace: ns, Name: name, PullCount: r.PullCount}:
			newCount++
		default:
		}
	}
	return newCount
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
