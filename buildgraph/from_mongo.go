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

	// Canais de orquestração
	repoChan := make(chan *myutils.Repository, 1000)
	jobChan := make(chan GraphJob, 5000)
	doneChan := make(chan struct{})

	var wgLoad sync.WaitGroup
	var wgBuild sync.WaitGroup

	// 1. Loader: Lê do MongoDB e joga no repoChan
	go func() {
		loadReposToChannel(page, pageSize, pullCountThreshold, repoChan)
		close(repoChan)
	}()

	// 2. Repo Workers: Processam Tags e Manifestos em paralelo
	numRepoWorkers := runtime.NumCPU() * 2
	for i := 0; i < numRepoWorkers; i++ {
		wgLoad.Add(1)
		go func() {
			defer wgLoad.Done()
			repoWorker(repoChan, jobChan, tagCnt)
		}()
	}

	// 3. Build Workers: Inserem no Neo4j
	numBuildWorkers := runtime.NumCPU()
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
		filter := bson.M{"pull_count": bson.M{"$gte": threshold}}
		repos, err := myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(filter, repoPage, pageSize)
		if err != nil || len(repos) == 0 {
			break
		}

		for _, r := range repos {
			ch <- r
		}
		repoPage++
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

		for _, tag := range tags {
			imgs, err := myutils.ReqImagesMetadata(repo.Namespace, repo.Name, tag.Name)
			if err != nil {
				continue
			}

			// Persist image metadata to MongoDB so the rank stage (Stage III)
			// can compute Neo4j node IDs without live API calls.
			for _, img := range imgs {
				if myutils.GlobalDBClient.MongoFlag {
					if err := myutils.GlobalDBClient.Mongo.UpdateImage(img); err != nil {
						myutils.Logger.Error(fmt.Sprintf("UpdateImage %s failed: %v", img.Digest, err))
					}
				}
			}

			// Persist tag to MongoDB (with its Images array set).
			// This is required by calculate_node_dependent_weights loadDataFromMongo.
			tag.Images = make([]myutils.ImageInTag, 0, len(imgs))
			for _, img := range imgs {
				tag.Images = append(tag.Images, myutils.ImageInTag{
					Architecture: img.Architecture,
					OS:           img.OS,
					Digest:       img.Digest,
					Size:         img.Size,
				})
			}
			if myutils.GlobalDBClient.MongoFlag {
				if err := myutils.GlobalDBClient.Mongo.UpdateTag(tag); err != nil {
					myutils.Logger.Error(fmt.Sprintf("UpdateTag %s/%s:%s failed: %v", repo.Namespace, repo.Name, tag.Name, err))
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
					TagName:       tag.Name,
					ImageMeta:     img,
				}
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
