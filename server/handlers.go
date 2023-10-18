package server

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"myutils"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// =========================================================
// currently only used for response to RepositoriesView
// =========================================================

// RepositoryForServer only used for reply from this backend server
type RepositoryForServer struct {
	User            string         `json:"user"`
	Name            string         `json:"name"`
	Namespace       string         `json:"namespace"`
	RepositoryType  string         `json:"repository_type"`
	Description     string         `json:"description"`
	IsPrivate       bool           `json:"is_private"`
	IsAutomated     bool           `json:"is_automated"`
	StarCount       int            `json:"star_count"`
	PullCount       int64          `json:"pull_count"`
	LastUpdated     string         `json:"last_updated"`
	DateRegistered  string         `json:"date_registered"`
	FullDescription string         `json:"full_description"`
	Tags            []TagForServer `json:"tags"`
}

// TagForServer only used for reply from this backend server
type TagForServer struct {
	Name                string           `json:"tag_name"`
	LastUpdated         string           `json:"last_updated"`
	LastUpdaterUsername string           `json:"last_updater_username"`
	TagLastPulled       string           `json:"tag_last_pulled"`
	TagLastPushed       string           `json:"tag_last_pushed"`
	MediaType           string           `json:"media_type"`
	ContentType         string           `json:"content_type"`
	Images              []ImageForServer `json:"images"`
}

// ImageForServer only used for reply from this backend server
type ImageForServer struct {
	Architecture string `json:"architecture"`
	Variant      string `json:"variant"`
	Digest       string `json:"digest"`
}

func RepositoriesToForServer(repos []*myutils.RepositoryOld) []*RepositoryForServer {
	var res = make([]*RepositoryForServer, 0)

	for _, repo := range repos {
		tmpRepo := &RepositoryForServer{
			User: repo.User, Name: repo.Name, Namespace: repo.Namespace,
			RepositoryType: repo.RepositoryType, Description: repo.Description,
			IsPrivate: repo.IsPrivate, IsAutomated: repo.IsAutomated,
			StarCount: repo.StarCount, PullCount: repo.PullCount,
			LastUpdated: repo.LastUpdated, DateRegistered: repo.DateRegistered,
			FullDescription: repo.FullDescription, Tags: make([]TagForServer, 0),
		}

		for tagName, tagMeta := range repo.Tags {
			tmpTag := TagForServer{
				Name: tagName, LastUpdated: tagMeta.LastUpdated, LastUpdaterUsername: tagMeta.LastUpdaterUsername,
				TagLastPulled: tagMeta.TagLastPulled, TagLastPushed: tagMeta.TagLastPushed,
				MediaType: tagMeta.MediaType, ContentType: tagMeta.ContentType,
				Images: make([]ImageForServer, 0),
			}
			for arch, i := range tagMeta.Images {
				for variant, digest := range i {
					tmpImage := ImageForServer{
						Architecture: arch, Variant: variant, Digest: digest,
					}

					tmpTag.Images = append(tmpTag.Images, tmpImage)
				}
			}

			tmpRepo.Tags = append(tmpRepo.Tags, tmpTag)
		}

		res = append(res, tmpRepo)
	}

	return res
}

// =========================================================

// handleRepositoriesSearch return a function used for
// repositories searching API exported by gin framework
//
// URI arguments:
//
// search: keyword for searching repositories from MongoDB,
//
//	now search according to name > namespace > description > full_description
//
// page: current page of the view
//
// page_size: page size of the view
func handleRepositoriesSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		totalCnt := totalRepositoriesCnt
		if search != "" {
			// time costs too much
			totalCnt, _ = myMongo.GetRepositoriesCountByText(search)
		}
		results, err := myMongo.FindRepositoriesByText(search, int64(page), int64(pageSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.Header("Access-Control-Allow-Origin", "*")
			c.JSON(http.StatusOK, gin.H{
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   RepositoriesToForServer(results),
			})
		}
	}
}

// =========================================================
// used for mapping analysis results of images to layers
// =========================================================

type ImageWithResults struct {
	Architecture string             `json:"architecture"`
	Features     string             `json:"features"`
	Variant      string             `json:"variant"`
	Digest       string             `json:"digest"`
	Layers       []LayerWithResults `json:"layers"`
	OS           string             `json:"os"`
	Size         int64              `json:"size"`
	Status       string             `json:"status"`
	LastPulled   string             `json:"last_pulled"`
	LastPushed   string             `json:"last_pushed"`
}

type LayerWithResults struct {
	Digest      string `json:"digest,omitempty"`
	Size        int    `json:"size"`
	Instruction string `json:"instruction"`
	Results     string `json:"results"`
}

func ImagesToWithResults(imgs []*myutils.ImageOld) []*ImageWithResults {
	imgWithRes := make([]*ImageWithResults, 0)

	for _, img := range imgs {
		results, err := myMongo.FindResultByDigest(img.Digest)
		if err != nil {
			continue
		}

		tmp := combineImageAndResult(img, results)
		imgWithRes = append(imgWithRes, tmp)
	}

	return imgWithRes
}

func combineImageAndResult(img *myutils.ImageOld, res *myutils.ImageResult) *ImageWithResults {
	imgres := &ImageWithResults{
		Architecture: img.Architecture, Features: img.Features, Variant: img.Variant,
		Digest: img.Digest, OS: img.OS, Size: img.Size, Status: img.Status,
		LastPulled: img.LastPulled, LastPushed: img.LastPushed,
	}

	// add layers of img to imgres
	for _, layer := range img.Layers {
		tmpLayer := LayerWithResults{}
		b, _ := json.Marshal(layer)
		json.Unmarshal(b, &tmpLayer)
		imgres.Layers = append(imgres.Layers, tmpLayer)
	}

	// regex match
	re, _ := regexp.Compile(`layer\[(\d+)]\.instruction`)

	// add results to layer according to result.Path
	for _, result := range res.Results {
		if result.Type == "in-dockerfile-command" {
			layerIndex := re.FindStringSubmatch(result.Path)
			if len(layerIndex) > 1 {
				index, err := strconv.Atoi(layerIndex[1])
				if err != nil {
					continue
				}
				b, err := json.Marshal(result)
				if err != nil {
					continue
				}
				imgres.Layers[index].Results = string(b)
			}
		}
	}

	return imgres
}

// =========================================================
// handleImagesSearch return a function used for images
// searching API exported by gin framework
//
// URI arguments:
// search: keyword for searching images from MongoDB,
//
//	now only searching according to digest
//
// page: current page of the view
//
// page_size: page size of the view
func handleImagesSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		totalCnt := totalImagesCnt
		if search != "" && !strings.Contains(search, "sha256:") {
			// time costs too much
			totalCnt, _ = myMongo.GetImagesCountByText(search)
		}
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		results, err := myMongo.FindImagesByText(search, int64(page), int64(pageSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.Header("Access-Control-Allow-Origin", "*")
			c.JSON(http.StatusOK, gin.H{
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   ImagesToWithResults(results),
			})
		}
	}
}

func ResultsToImagesWithResults(imgRes []*myutils.ImageResult) []*ImageWithResults {
	imgWithRes := make([]*ImageWithResults, 0)

	for _, res := range imgRes {
		img, err := myMongo.FindImageByDigest(res.Digest)
		if err != nil {
			continue
		}
		tmp := combineImageAndResult(img, res)
		imgWithRes = append(imgWithRes, tmp)
	}

	return imgWithRes
}

// =========================================================
// handleResultSearch return a function used for results
// searching API exported by gin framework
//
// URI arguments:
// search: keyword for searching images from MongoDB,
//
//	now only searching according to digest
//
// page: current page of the view
//
// page_size: page size of the view
func handleResultSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		totalCnt := totalImagesCnt
		if search != "" {
			// time costs too much
			totalCnt, _ = myMongo.GetResultsCountByText(search)
		}
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		results, err := myMongo.FindResultsByText(search, int64(page), int64(pageSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.Header("Access-Control-Allow-Origin", "*")
			c.JSON(http.StatusOK, gin.H{
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   ResultsToImagesWithResults(results),
			})
		}
	}
}
