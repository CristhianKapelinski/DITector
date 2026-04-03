package buildgraph

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
	"go.mongodb.org/mongo-driver/bson"
)

type GraphJob struct {
	Registry      string
	RepoNamespace string
	RepoName      string
	TagName       string
	ImageMeta     *myutils.Image
}

// StartFromMongo inicia o processamento paralelo do grafo a partir do MongoDB
func StartFromMongo(page int64, pageSize int64, tagCnt int, pullCountThreshold int64) {
	beginTime := time.Now()
	fmt.Println("Build paralelo iniciado em:", myutils.GetLocalNowTimeStr())

	// Canais de orquestração. Buffers grandes evitam bloqueio entre estágios.
	repoChan := make(chan *myutils.Repository, 4000)
	jobChan := make(chan GraphJob, 20000)
	doneChan := make(chan struct{})

	var wgLoad sync.WaitGroup
	var wgBuild sync.WaitGroup

	// 1. Loader: Lê do MongoDB e joga no repoChan
	go func() {
		loadReposToChannel(page, pageSize, pullCountThreshold, repoChan)
		close(repoChan)
	}()

	// 2. Repo Workers: buscam tags e manifestos via Docker Hub API (I/O bound).
	// Escalamos muito acima de NumCPU pois a maior parte do tempo é espera de rede.
	numRepoWorkers := runtime.NumCPU() * 16
	if numRepoWorkers < 64 {
		numRepoWorkers = 64
	}
	for i := 0; i < numRepoWorkers; i++ {
		wgLoad.Add(1)
		go func() {
			defer wgLoad.Done()
			repoWorker(repoChan, jobChan, tagCnt)
		}()
	}

	// 3. Build Workers: inserem no Neo4j (I/O bound — Bolt protocol over TCP).
	numBuildWorkers := runtime.NumCPU() * 4
	if numBuildWorkers < 16 {
		numBuildWorkers = 16
	}
	for i := 0; i < numBuildWorkers; i++ {
		wgBuild.Add(1)
		go func() {
			defer wgBuild.Done()
			buildGraphWorker(jobChan)
		}()
	}

	// Fechamento em cascata
	go func() {
		wgLoad.Wait()
		close(jobChan)
		wgBuild.Wait()
		close(doneChan)
	}()

	<-doneChan
	fmt.Printf("Build finalizado. Tempo total: %v\n", time.Since(beginTime))
}

func loadReposToChannel(page int64, pageSize int64, threshold int64, ch chan *myutils.Repository) {
	repoPage := page
	for {
		// Resume: only load repos that have NOT been fully processed by a
		// previous run. graph_built_at is set by repoWorker after all tags
		// and images for the repo have been inserted into Neo4j.
		filter := bson.M{
			"pull_count":     bson.M{"$gte": threshold},
			"graph_built_at": bson.M{"$exists": false},
		}
		// Sort by pull_count descending to prioritize influential images
		opts := mongodb_opts.Find().SetSort(bson.M{"pull_count": -1})
		cursor, err := myutils.GlobalDBClient.Mongo.RepoColl.Find(context.Background(), filter, opts)
		if err != nil {
			break
		}
		defer cursor.Close(context.Background())

		for cursor.Next(context.Background()) {
			var r myutils.Repository
			if err := cursor.Decode(&r); err != nil {
				continue
			}
			ch <- &r
		}
		break 
	}
}

// networkKeywords are used as a heuristic to identify containers that
// likely expose network services and are therefore candidates for OpenVAS scanning.
var networkKeywords = []string{
	"nginx", "apache", "http", "https", "server", "web", "api", "rest",
	"grpc", "db", "database", "mysql", "postgres", "sql", "redis", "mongo",
	"elastic", "kafka", "rabbitmq", "proxy", "gateway", "lb", "balancer",
	"vpn", "ssh", "ftp", "smtp", "imap", "ldap", "app", "service", "svc",
}

func isNetworkContainer(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range networkKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// tagConcurrency limits concurrent image-manifest fetches per repo to avoid
// triggering per-repo rate limits on Docker Hub.
const tagConcurrency = 4

func repoWorker(repoChan chan *myutils.Repository, jobChan chan GraphJob, tagCnt int) {
	for repo := range repoChan {
		// Only submit to the graph if the repo passes the network heuristic.
		// The pull_count threshold is already enforced at the MongoDB query level
		// in loadReposToChannel, so no secondary check is needed here.
		if !isNetworkContainer(repo.Name) {
			continue
		}

		tags, err := myutils.ReqTagsMetadata(repo.Namespace, repo.Name, 1, tagCnt)
		if err != nil {
			continue
		}

		// Fetch image manifests for all tags in parallel. Each ReqImagesMetadata
		// call is an independent HTTPS round-trip; serialising them wastes wall
		// time proportional to (numTags × latency).
		sem := make(chan struct{}, tagConcurrency)
		var wg sync.WaitGroup
		for _, tag := range tags {
			wg.Add(1)
			sem <- struct{}{}
			go func(t *myutils.Tag) {
				defer wg.Done()
				defer func() { <-sem }()

				imgs, err := myutils.ReqImagesMetadata(repo.Namespace, repo.Name, t.Name)
				if err != nil {
					return
				}

				// Persist image metadata to MongoDB so Stage III can compute
				// Neo4j node IDs without live API calls.
				for _, img := range imgs {
					if myutils.GlobalDBClient.MongoFlag {
						if err := myutils.GlobalDBClient.Mongo.UpdateImage(img); err != nil {
							myutils.Logger.Error(fmt.Sprintf("UpdateImage %s failed: %v", img.Digest, err))
						}
					}
				}

				// Persist tag to MongoDB (with its Images array populated).
				// Required by calculate_node_dependent_weights loadDataFromMongo.
				t.Images = make([]myutils.ImageInTag, 0, len(imgs))
				for _, img := range imgs {
					t.Images = append(t.Images, myutils.ImageInTag{
						Architecture: img.Architecture,
						OS:           img.OS,
						Digest:       img.Digest,
						Size:         img.Size,
					})
				}
				if myutils.GlobalDBClient.MongoFlag {
					if err := myutils.GlobalDBClient.Mongo.UpdateTag(t); err != nil {
						myutils.Logger.Error(fmt.Sprintf("UpdateTag %s/%s:%s failed: %v", repo.Namespace, repo.Name, t.Name, err))
					}
				}

				for _, img := range imgs {
					if img.OS == "windows" {
						continue
					}
					jobChan <- GraphJob{
						Registry:      "docker.io",
						RepoNamespace: repo.Namespace,
						RepoName:      repo.Name,
						TagName:       t.Name,
						ImageMeta:     img,
					}
				}
			}(tag)
		}
		wg.Wait()

		// All tags processed: mark repo as graph-built so it is skipped on restart.
		if myutils.GlobalDBClient.MongoFlag {
			if err := myutils.GlobalDBClient.Mongo.MarkRepoGraphBuilt(repo.Namespace, repo.Name); err != nil {
				myutils.Logger.Error(fmt.Sprintf("MarkRepoGraphBuilt %s/%s failed: %v", repo.Namespace, repo.Name, err))
			}
		}
	}
}

func buildGraphWorker(jobChan chan GraphJob) {
	for job := range jobChan {
		id := fmt.Sprintf("%s/%s/%s:%s@%s", job.Registry, job.RepoNamespace, job.RepoName, job.TagName, job.ImageMeta.Digest)
		myutils.GlobalDBClient.Neo4j.InsertImageToNeo4j(id, job.ImageMeta)
		myutils.Logger.Info(fmt.Sprintf("Inserido no Neo4j: %s", id))
	}
}
