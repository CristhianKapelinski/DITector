package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Musso12138/dockercrawler/myutils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"os"
	"path"
	"strings"
)

type CurrentImage struct {
	dockerClient *client.Client

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
	metadata       *metadata
	recommendedCmd []string

	// configuration of the image
	configuration   *Configuration
	defaultCmd      defaultCmd
	defaultExecFile []string // filepath of default executed files

	// content of the image
	imgTarFile                 string // filepath of image tar
	imgFilepath                string // filepath of uncompressed image file
	manifest                   manifest
	layerWithContentList       []string
	layerLocalRootFilepathList []string // /.../layer-id
	layerLocalFilepathList     []string // /.../layer-id/layer
	layerInfoMap               map[string]*layerInfo
}

type metadata struct {
	repositoryMetadata *myutils.Repository
	tagMetadata        *myutils.Tag
	imageMetadata      *myutils.Image
}

type defaultCmd struct {
	entrypoint string
	cmd        string
	fullCmd    string
}

type layerInfo struct {
	size        int64
	instruction string
	digest      string

	localRootFilePath string // parent dir path of localFilePath
	localFilePath     string // localFilePath of the layer
}

type imagePullEvent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Progress string `json:"progress"`
}

type downloadFinish struct {
	imgTarPath string // filepath for the tar archive
	imgDirPath string // filepath for the extracted result dir
	err        error
}

type manifests []manifest

type manifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func NewCurrentImage(imgName string) (*CurrentImage, error) {
	currI := new(CurrentImage)
	var err error

	currI.dockerClient, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	currI.name = imgName
	currI.parseName()

	// 初始化引用变量
	currI.metadata = new(metadata)
	currI.configuration = new(Configuration)
	currI.layerWithContentList = make([]string, 0)
	currI.layerLocalRootFilepathList = make([]string, 0)
	currI.layerLocalFilepathList = make([]string, 0)
	currI.layerInfoMap = make(map[string]*layerInfo)
	currI.manifest = manifest{}

	return currI, nil
}

// pullSaveExtractImage pulls Docker image to local Docker env, saves it
// to a tar archive, and extracts all tar archive(including image and each layer).
func (currI *CurrentImage) pullSaveExtractImage(targetDir string, finish chan downloadFinish) {
	var imgTarPath string
	var imgDirPath string
	var err error

	defer func() {
		finish <- downloadFinish{imgTarPath: imgTarPath, imgDirPath: imgDirPath, err: err}
	}()

	myutils.Logger.Debug("start to pull, save and extract image", currI.name)

	// 同步下载镜像
	if err = currI.pullImage(); err != nil {
		myutils.Logger.Error("pull image", currI.name, "failed with:", err.Error())
		return
	}

	// 保存镜像
	targetTarFilename := fmt.Sprintf("%s-%s-%s.tar", currI.namespace, currI.repoName, currI.tagName)
	imgTarPath = path.Join(targetDir, targetTarFilename)
	if err = currI.saveImage(imgTarPath); err != nil {
		myutils.Logger.Error("save image", currI.name, "to file", imgTarPath, "failed with:", err.Error())
		return
	}

	// 解压镜像
	targetDirname := fmt.Sprintf("%s-%s-%s", currI.namespace, currI.repoName, currI.tagName)
	imgDirPath = path.Join(targetDir, targetDirname)
	if err = currI.extractImage(imgTarPath, imgDirPath); err != nil {
		myutils.Logger.Error("extract image", currI.name, "from file", imgTarPath, "failed with:", err.Error())
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
		event := new(imagePullEvent)
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

// extractImage extracts source image tar archive to dest dir,
// including image tar and all layer tar.
func (currI *CurrentImage) extractImage(imgTar, dstDir string) error {
	// 解压image tar
	if err := myutils.ExtractTar(imgTar, dstDir); err != nil {
		return err
	}

	// 加载manifest
	manifestFilepath := path.Join(dstDir, "manifest.json")
	manifestFile, err := os.ReadFile(manifestFilepath)
	if err != nil {
		return err
	}
	mf := manifests{}
	if err = json.Unmarshal(manifestFile, &mf); err != nil {
		return err
	}
	if len(mf) == 0 {
		return fmt.Errorf("no manifest in file %s", manifestFilepath)
	}
	currI.manifest = mf[0]

	// 逐个解压layer tar
	for _, layerTarFilename := range currI.manifest.Layers {
		layerTarFilepath := path.Join(dstDir, layerTarFilename)
		digest := strings.Split(layerTarFilename, "/")[0]
		layerRootFilepath := path.Join(dstDir, digest)
		layerFilepath := path.Join(dstDir, digest, "layer")
		if err = myutils.ExtractTar(layerTarFilepath, layerFilepath); err != nil {
			return err
		}
		// 将解压得到的本地layer文件夹维护起来
		currI.layerLocalRootFilepathList = append(currI.layerLocalRootFilepathList, layerRootFilepath)
		currI.layerLocalFilepathList = append(currI.layerLocalFilepathList, layerFilepath)
	}

	return nil
}
