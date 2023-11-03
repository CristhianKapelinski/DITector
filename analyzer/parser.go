package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"myutils"
	"os"
	"path"
	"strings"
)

type CurrentImage struct {
	dockerClient *client.Client
	tarFilepath  string
	filepath     string
	name         string

	registry     string
	namespace    string
	repoName     string
	tagName      string
	architecture string
	variant      string
	os           string
	osVersion    string
	digest       string

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

type downloadFinish struct {
	tarFilepath string // filepath for the tar archive
	filepath    string // filepath for the extracted result dir
	err         error
}

// pullSaveExtractImage pulls Docker image to local Docker env, saves it
// to a tar archive, and extracts all tar archive(including image and each layer).
func (currI *CurrentImage) pullSaveExtractImage(dir string, finish chan downloadFinish) {
	var tarFilepath string
	var filepath string
	var err error

	defer func() {
		finish <- downloadFinish{tarFilepath: tarFilepath, filepath: filepath, err: err}
	}()

	myutils.Logger.Debug("start pulling image", currI.name)

	// 同步下载镜像
	if err = currI.pullImage(); err != nil {
		myutils.Logger.Error("pull image", currI.name, "failed with:", err.Error())
		return
	}

	// 保存镜像
	targetTarFilename := fmt.Sprintf("%s-%s-%s.tar.gz", currI.namespace, currI.repoName, currI.tagName)
	tarFilepath = path.Join(myutils.GlobalConfig.TmpDir, targetTarFilename)
	if err = currI.saveImage(tarFilepath); err != nil {
		myutils.Logger.Error("save image", currI.name, "to filepath", tarFilepath, "failed with:", err.Error())
		return
	}

	// 解压镜像
	targetDirname := fmt.Sprintf("%s-%s-%s", currI.namespace, currI.repoName, currI.tagName)
	filepath = path.Join(myutils.GlobalConfig.TmpDir, targetDirname)
	if err = extractImage(tarFilepath, filepath); err != nil {
		myutils.Logger.Error("extract image", currI.name, "from file", tarFilepath, "failed with:", err.Error())
		return
	}

	return
}

// pullImage calls client.Client.ImagePull to download image.
// It turns ImagePull progress from async to sync with a non-buffered chan.
func (currI *CurrentImage) pullImage() error {
	rc, err := currI.dockerClient.ImagePull(context.TODO(), currI.name, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()

	success := false

	decoder := json.NewDecoder(rc)
	for {
		event := new(ImagePullEvent)
		if err = decoder.Decode(event); err != nil {
			if err == io.EOF {
				break
			}
			myutils.Logger.Error("decode JSON when pulling image", currI.name, "failed with:", err.Error())
		}

		if strings.Contains(event.Status, "Downloaded newer image for") ||
			strings.Contains(event.Status, "Image is up to date") {
			success = true
		}
	}

	if success {
		return nil
	} else {
		return fmt.Errorf("not catch download success signal in ImagePull events")
	}
}

// saveImage calls client.Client.ImageSave to save image to tar archive.
func (currI *CurrentImage) saveImage(filepath string) error {
	imageRC, err := currI.dockerClient.ImageSave(context.TODO(), []string{currI.name})
	if err != nil {
		return err
	}
	defer imageRC.Close()

	tarFile, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	_, err = io.Copy(tarFile, imageRC)
	if err != nil {
		return err
	}

	return nil
}

// extractImage TODO: extracts source image tar archive to dest dir.
func extractImage(source, dest string) error {
	return nil
}
