package buildgraph

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
)

type cpEntry struct {
	Namespace string `json:"ns"`
	Name      string `json:"name"`
	BuiltAt   string `json:"built_at"`
	Tags      int    `json:"tags"`
}

// StartFromMongo runs the Stage II distributed pipeline.
//
// Multiple machines can run this concurrently — ClaimNextBuildRepo is atomic,
// so no two workers ever process the same repo. On restart, ResetStaleBuildClaims
// recovers any repos that were mid-flight when the previous run crashed.
//
// Progress is checkpointed to dataDir/build_checkpoint.jsonl (host-mounted path)
// so it survives container restarts independently of the Docker daemon.
// Metrics are written every 60s to dataDir/build_metrics.log with ETA.
func StartFromMongo(threshold int64, workers int, ip myutils.IdentityProvider, dataDir string) {
	if myutils.GlobalDBClient.MongoFlag {
		myutils.GlobalDBClient.Mongo.ResetStaleBuildClaims()
	}

	m := newBuildMetrics(threshold)
	metricsDone := make(chan struct{})
	m.startReporter(dataDir, metricsDone)

	cpCh := make(chan cpEntry, 1000)
	go checkpointWriter(cpCh, dataDir)

	numRepo := workers
	if numRepo <= 0 {
		numRepo = 1
	}
	var wgRepo sync.WaitGroup
	for i := 0; i < numRepo; i++ {
		wgRepo.Add(1)
		go func() {
			defer wgRepo.Done()
			repoWorker(myutils.NewHubClient(ip), threshold, cpCh, m)
		}()
	}

	wgRepo.Wait()
	close(cpCh)
	close(metricsDone)
}

// repoWorker claims repos one at a time and processes them. Mirrors Stage I's
// immortal-worker pattern: only exits when the queue is confirmed empty.
func repoWorker(hub *myutils.HubClient, threshold int64, cpCh chan<- cpEntry, m *BuildMetrics) {
	emptyCount := 0
	for {
		repo, err := myutils.GlobalDBClient.Mongo.ClaimNextBuildRepo(threshold)
		if err != nil || repo == nil {
			emptyCount++
			if emptyCount%6 == 0 {
				count, _ := myutils.GlobalDBClient.Mongo.CountPendingBuildRepos(threshold)
				if count == 0 {
					break
				}
			}
			time.Sleep(5 * time.Second)
			continue
		}
		emptyCount = 0
		
		// ATOMIC FIX: Only mark as built if processing (API + Neo4j) succeeds.
		if processRepo(hub, repo, cpCh, m) {
			markBuilt(repo, cpCh, m)
		} else {
			myutils.Logger.Warn(fmt.Sprintf("!!! processRepo failed for %s/%s, cooling off 30s...", repo.Namespace, repo.Name))
			time.Sleep(30 * time.Second)
		}
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // anti-fingerprint jitter
	}
}

func processRepo(hub *myutils.HubClient, repo *myutils.Repository, cpCh chan<- cpEntry, m *BuildMetrics) bool {
	tags := getTags(hub, repo, m)
	if tags == nil {
		return false
	}

	for _, tag := range tags {
		time.Sleep(time.Duration(400+rand.Intn(500)) * time.Millisecond) // jitter between tag requests
		imgs, err := getImages(hub, repo, tag, m)
		if err != nil {
			m.Errors.Add(1)
			myutils.Logger.Warn(fmt.Sprintf("getImages %s/%s:%s: %v", repo.Namespace, repo.Name, tag.Name, err))
			return false // Fail atomic process on API error
		}
		for _, img := range imgs {
			if img.OS == "windows" {
				continue
			}
			
			// SYNC WRITE: The core of the atomic guarantee.
			id := fmt.Sprintf("docker.io/%s/%s:%s@%s", repo.Namespace, repo.Name, tag.Name, img.Digest)
			if err := myutils.GlobalDBClient.Neo4j.InsertImageToNeo4j(id, img); err != nil {
				m.Errors.Add(1)
				myutils.Logger.Error(fmt.Sprintf("Neo4j Sync Error for %s: %v", id, err))
				return false // Fail the whole repo if graph insertion fails
			}
			m.Neo4jInserts.Add(1)
		}
	}
	return true
}

func markBuilt(repo *myutils.Repository, cpCh chan<- cpEntry, m *BuildMetrics) {
	if !myutils.GlobalDBClient.MongoFlag {
		return
	}
	if err := myutils.GlobalDBClient.Mongo.MarkRepoGraphBuilt(repo.Namespace, repo.Name); err != nil {
		myutils.Logger.Error(fmt.Sprintf("MarkRepoGraphBuilt %s/%s: %v", repo.Namespace, repo.Name, err))
		return
	}
	m.Processed.Add(1)
	cpCh <- cpEntry{
		Namespace: repo.Namespace,
		Name:      repo.Name,
		BuiltAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

func getTags(hub *myutils.HubClient, repo *myutils.Repository, m *BuildMetrics) []*myutils.Tag {
	if myutils.GlobalDBClient.MongoFlag {
		tags, err := myutils.GlobalDBClient.Mongo.FindAllTagsByRepoName(repo.Namespace, repo.Name)
		if err == nil && allTagsHaveImages(tags) {
			m.TagCacheHits.Add(1)
			return tags
		}
	}
	// Always fetch the most recently updated tag (page 1, size 1) plus
	// "latest" explicitly. Deduplicate in case they are the same tag.
	recent, err := hub.GetTags(repo.Namespace, repo.Name, 1, 1)
	if err != nil {
		m.Errors.Add(1)
		myutils.Logger.Warn(fmt.Sprintf("GetTags %s/%s: %v", repo.Namespace, repo.Name, err))
		return nil
	}
	m.TagAPIFetches.Add(1)

	tags := recent
	if len(recent) == 0 || recent[0].Name != "latest" {
		latest, err := hub.GetTag(repo.Namespace, repo.Name, "latest")
		if err != nil {
			myutils.Logger.Warn(fmt.Sprintf("GetTag %s/%s:latest: %v", repo.Namespace, repo.Name, err))
		} else if latest != nil {
			tags = append(tags, latest)
		}
	}

	if myutils.GlobalDBClient.MongoFlag {
		for _, t := range tags {
			if err := myutils.GlobalDBClient.Mongo.UpdateTag(t); err != nil {
				myutils.Logger.Error(fmt.Sprintf("UpdateTag %s/%s:%s: %v", repo.Namespace, repo.Name, t.Name, err))
			}
		}
	}
	return tags
}

func getImages(hub *myutils.HubClient, repo *myutils.Repository, t *myutils.Tag, m *BuildMetrics) ([]*myutils.Image, error) {
	if myutils.GlobalDBClient.MongoFlag && len(t.Images) > 0 {
		if imgs, ok := loadImagesFromCache(t.Images); ok {
			m.ImageCacheHits.Add(1)
			return imgs, nil
		}
	}
	imgs, err := hub.GetImages(repo.Namespace, repo.Name, t.Name)
	if err != nil {
		return nil, err
	}
	m.ImageAPIFetches.Add(1)
	if myutils.GlobalDBClient.MongoFlag {
		persistImages(repo, t, imgs)
	}
	return imgs, nil
}

func persistImages(repo *myutils.Repository, t *myutils.Tag, imgs []*myutils.Image) {
	for _, img := range imgs {
		if err := myutils.GlobalDBClient.Mongo.UpdateImage(img); err != nil {
			myutils.Logger.Error(fmt.Sprintf("UpdateImage %s: %v", img.Digest, err))
		}
	}
	// t.Images already carries the full ImageInTag payload from the tags API
	// (Features, Variant, Status, LastPulled, LastPushed, etc.) — do not overwrite.
	if err := myutils.GlobalDBClient.Mongo.UpdateTag(t); err != nil {
		myutils.Logger.Error(fmt.Sprintf("UpdateTag %s/%s:%s: %v", repo.Namespace, repo.Name, t.Name, err))
	}
}

func allTagsHaveImages(tags []*myutils.Tag) bool {
	if len(tags) == 0 {
		return false
	}
	for _, t := range tags {
		if len(t.Images) == 0 {
			return false
		}
	}
	return true
}

func loadImagesFromCache(refs []myutils.ImageInTag) ([]*myutils.Image, bool) {
	digests := make([]string, 0, len(refs))
	for _, ref := range refs {
		digests = append(digests, ref.Digest)
	}
	byDigest, err := myutils.GlobalDBClient.Mongo.FindImagesByDigests(digests)
	if err != nil {
		return nil, false
	}
	imgs := make([]*myutils.Image, 0, len(refs))
	for _, ref := range refs {
		img, ok := byDigest[ref.Digest]
		if !ok || len(img.Layers) == 0 {
			return nil, false
		}
		imgs = append(imgs, img)
	}
	return imgs, true
}

func checkpointWriter(ch <-chan cpEntry, dir string) {
	path := filepath.Join(dir, "build_checkpoint.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		myutils.Logger.Error(fmt.Sprintf("checkpoint open %s: %v", path, err))
		for range ch {}
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for e := range ch {
		_ = enc.Encode(e)
	}
}
