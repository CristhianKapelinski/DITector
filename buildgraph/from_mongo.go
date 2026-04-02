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

func repoWorker(repoChan chan *myutils.Repository, jobChan chan GraphJob, tagCnt int) {
	networkKeywords := []string{"nginx", "http", "server", "api", "db", "database", "sql", "redis", "proxy", "app"}

	for repo := range repoChan {
		isNetworkRepo := false
		lowerName := strings.ToLower(repo.Name)
		for _, kw := range networkKeywords {
			if strings.Contains(lowerName, kw) {
				isNetworkRepo = true
				break
			}
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

			for _, img := range imgs {
				if img.OS == "windows" { continue }
				
				if isNetworkRepo || repo.PullCount > 10000 {
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
}

func buildGraphWorker(jobChan chan GraphJob) {
	for job := range jobChan {
		id := fmt.Sprintf("%s/%s/%s:%s@%s", job.Registry, job.RepoNamespace, job.RepoName, job.TagName, job.ImageMeta.Digest)
		myutils.GlobalDBClient.Neo4j.InsertImageToNeo4j(id, job.ImageMeta)
		myutils.Logger.Info(fmt.Sprintf("Inserido no Neo4j: %s", id))
	}
}
