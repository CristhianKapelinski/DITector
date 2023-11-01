package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"myutils"
	"strings"
)

type CurrentImage struct {
	dockerClient *client.Client
	filepath     string
	name         string

	registry       string
	namespace      string
	repositoryName string
	tagName        string
	architecture   string
	variant        string
	os             string
	osVersion      string
	digest         string

	// metadata of the repository, the tag and the image
	metadata *metadata
	// configuration of the image
	configuration *types.ImageInspect
	// content of the image
	layerWithContentList []string
	layerInfoMap         map[string]layerInfo
}

type metadata struct {
	repositoryMetadata *myutils.Repository
	tagMetadata        *myutils.Tag
	imageMetadata      *myutils.Image
}

type layerInfo struct {
	size        int64
	instruction string
	digest      string
	// localFilePath of the layer
	localFilePath string
}

func NewCurrentImage() (*CurrentImage, error) {
	currI := new(CurrentImage)
	var err error

	currI.dockerClient, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	currI.metadata = new(metadata)
	currI.layerWithContentList = make([]string, 0)
	currI.layerInfoMap = make(map[string]layerInfo)

	return currI, nil
}

type ImagePullEvent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Progress string `json:"progress"`
}

// pullImage calls client.Client.ImagePull to downloads image.
// It turns ImagePull progress from async to sync with a non-buffered chan.
func (currI *CurrentImage) pullImage(ch chan<- bool) {
	myutils.Logger.Debug("start pulling image", currI.name)
	rc, err := currI.dockerClient.ImagePull(context.TODO(), currI.name, types.ImagePullOptions{})
	if err != nil {
		myutils.Logger.Error("pull image", currI.name, "failed with:", err.Error())
		ch <- false
		return
	}
	defer rc.Close()

	pullFlag := false

	decoder := json.NewDecoder(rc)
	for {
		event := new(ImagePullEvent)
		if err := decoder.Decode(event); err != nil {
			if err == io.EOF {
				break
			}
			myutils.Logger.Error("decode JSON when pulling image", currI.name, "failed with:", err.Error())
		}
		fmt.Println(event)
		if strings.Contains(event.Status, "Downloaded newer image for") ||
			strings.Contains(event.Status, "Image is up to date") {
			pullFlag = true
		}
	}

	if pullFlag {
		myutils.Logger.Debug("pull image", currI.name, "succeeded")
		ch <- true
	} else {
		ch <- false
	}

	return
}

// TODO: downloadImage downloads tar file of image directly from Docker Hub
// to specific local directory.
func (currI *CurrentImage) downloadImage(dir string) {

}
