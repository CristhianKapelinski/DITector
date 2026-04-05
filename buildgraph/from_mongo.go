package buildgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
)

type GraphJob struct {
	Registry      string
	RepoNamespace string
	RepoName      string
	TagName       string
	ImageMeta     *myutils.Image
}

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
func StartFromMongo(tagCnt int, threshold int64, ip myutils.IdentityProvider, dataDir string) {
	if myutils.GlobalDBClient.MongoFlag {
		myutils.GlobalDBClient.Mongo.ResetStaleBuildClaims()
	}

	m := newBuildMetrics(threshold)
	metricsDone := make(chan struct{})
	m.startReporter(dataDir, metricsDone)

	cpCh := make(chan cpEntry, 1000)
	jobChan := make(chan GraphJob, 10000)

	go checkpointWriter(cpCh, dataDir)

	numRepo := runtime.NumCPU() * 8
	if numRepo < 32 {
		numRepo = 32
	}
	var wgRepo sync.WaitGroup
	for i := 0; i < numRepo; i++ {
		wgRepo.Add(1)
		go func() {
			defer wgRepo.Done()
			repoWorker(myutils.NewHubClient(ip), threshold, tagCnt, jobChan, cpCh, m)
		}()
	}

	numGraph := runtime.NumCPU() * 2
	if numGraph < 8 {
		numGraph = 8
	}
	var wgGraph sync.WaitGroup
	for i := 0; i < numGraph; i++ {
		wgGraph.Add(1)
		go func() {
			defer wgGraph.Done()
			graphWorker(jobChan, m)
		}()
	}

	wgRepo.Wait()
	close(jobChan)
	close(cpCh)
	wgGraph.Wait()
	close(metricsDone)
}

// repoWorker claims repos one at a time and processes them. Mirrors Stage I's
// immortal-worker pattern: only exits when the queue is confirmed empty.
func repoWorker(hub *myutils.HubClient, threshold int64, tagCnt int, jobChan chan<- GraphJob, cpCh chan<- cpEntry, m *BuildMetrics) {
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
		processRepo(hub, repo, tagCnt, jobChan, cpCh, m)
	}
}

func processRepo(hub *myutils.HubClient, repo *myutils.Repository, tagCnt int, jobChan chan<- GraphJob, cpCh chan<- cpEntry, m *BuildMetrics) {
	tags := getTags(hub, repo, tagCnt, m)
	defer markBuilt(repo, len(tags), cpCh, m)

	for _, tag := range tags {
		imgs, err := getImages(hub, repo, tag, m)
		if err != nil {
			m.Errors.Add(1)
			myutils.Logger.Warn(fmt.Sprintf("getImages %s/%s:%s: %v", repo.Namespace, repo.Name, tag.Name, err))
			continue
		}
		for _, img := range imgs {
			if img.OS == "windows" {
				continue
			}
			jobChan <- GraphJob{
				Registry:      "docker.io",
				RepoNamespace: repo.Namespace,
				RepoName:      repo.Name,
				TagName:       tag.Name,
				ImageMeta:     img,
			}
		}
	}
}

func markBuilt(repo *myutils.Repository, tagCount int, cpCh chan<- cpEntry, m *BuildMetrics) {
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
		Tags:      tagCount,
	}
}

func getTags(hub *myutils.HubClient, repo *myutils.Repository, tagCnt int, m *BuildMetrics) []*myutils.Tag {
	if myutils.GlobalDBClient.MongoFlag {
		tags, err := myutils.GlobalDBClient.Mongo.FindAllTagsByRepoName(repo.Namespace, repo.Name)
		if err == nil && allTagsHaveImages(tags) {
			m.TagCacheHits.Add(1)
			return tags
		}
	}
	tags, err := hub.GetTags(repo.Namespace, repo.Name, 1, tagCnt)
	if err != nil {
		m.Errors.Add(1)
		myutils.Logger.Warn(fmt.Sprintf("GetTags %s/%s: %v", repo.Namespace, repo.Name, err))
		return nil
	}
	m.TagAPIFetches.Add(1)
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
	t.Images = make([]myutils.ImageInTag, len(imgs))
	for i, img := range imgs {
		t.Images[i] = myutils.ImageInTag{
			Architecture: img.Architecture,
			OS:           img.OS,
			Digest:       img.Digest,
			Size:         img.Size,
		}
	}
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

func graphWorker(jobChan <-chan GraphJob, m *BuildMetrics) {
	for job := range jobChan {
		id := fmt.Sprintf("%s/%s/%s:%s@%s", job.Registry, job.RepoNamespace, job.RepoName, job.TagName, job.ImageMeta.Digest)
		myutils.GlobalDBClient.Neo4j.InsertImageToNeo4j(id, job.ImageMeta)
		m.Neo4jInserts.Add(1)
		myutils.Logger.Debug(fmt.Sprintf("Neo4j: %s", id))
	}
}

// checkpointWriter persists completed repos to a JSONL file in dataDir.
// Single goroutine consuming a channel — no mutex needed.
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
