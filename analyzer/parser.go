package analyzer

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"myutils"
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

	localFlag    bool
	downloadFlag bool

	// metadata of the repository, the tag and the image
	metadata *metadata
	// configuration of the image
	configuration *types.ImageInspect
	// content of the image
	layerWithContentList []string
	layerInfoMap         map[string]layerInfo

	Results *myutils.ImageResult
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

// Parse TODO: 解析指定镜像的元数据、配置信息，下载镜像，定位镜像的各个层
func (currI *CurrentImage) Parse() error {
	// 解析镜像基本信息
	currI.parseName()

	// 获取当前平台
	if err := currI.getServerPlatform(); err != nil {
		myutils.Logger.Error("get Docker server platform failed with:", err.Error())
	}

	// 获取元数据
	if err := currI.parseMetadata(); err != nil {
		return err
	}

	// 解析配置信息
	// 检查image是否位于本地Docker环境中，如果不存在则下载镜像
	if err := currI.parseConfigurationFromDockerEnv(); err != nil {
		myutils.Logger.Error("inspect image", currI.name, "failed with:", err.Error())

		// 将镜像下载到本地
		// TODO: 目前是异步的，下面可能还是读不了
		rc, err := currI.dockerClient.ImagePull(context.TODO(), currI.name, types.ImagePullOptions{})
		myutils.Logger.Debug("pulling image", currI.name)
		if err != nil {
			myutils.Logger.Error("pull image", currI.name, "failed with:", err.Error())
		} else {
			// 下载成功
			defer rc.Close()
			currI.downloadFlag = true
		}
	} else {
		currI.localFlag = true
	}

	// 下载后尝试解析镜像信息
	if currI.downloadFlag {
		if err := currI.parseConfigurationFromDockerEnv(); err != nil {
			myutils.Logger.Error("inspect image", currI.name, "failed with:", err.Error())
		} else {
			currI.localFlag = true
		}
	}

	//

	return nil
}

// ParsePartial TODO: 解析指定镜像的元数据
func (currI *CurrentImage) ParsePartial() {

}

// parseName parses registry, namespace, repository, tag of the image according to name.
func (currI *CurrentImage) parseName() {
	currI.registry, currI.namespace, currI.repositoryName, currI.tagName = myutils.DivideImageName(currI.name)
}

// getServerPlatform gets platform of the host with Docker client.
func (currI *CurrentImage) getServerPlatform() error {
	if plf, err := currI.dockerClient.ServerVersion(context.TODO()); err != nil {
		return err
	} else {
		currI.architecture, currI.os = plf.Arch, plf.Os
	}

	return nil
}

// parseMetadata loads metadata of repository
func (currI *CurrentImage) parseMetadata() error {
	var err error
	currI.metadata = new(metadata)

	if currI.metadata.repositoryMetadata, err = currI.getRepositoryMetadata(); err != nil {
		myutils.Logger.Error("parse repository metadata of image", currI.name, "failed with:", err.Error())
		return err
	}

	if currI.metadata.tagMetadata, err = currI.getTagMetadata(); err != nil {
		myutils.Logger.Error("parse tag metadata of image", currI.name, "failed with:", err.Error())
		return err
	}

	if currI.metadata.imageMetadata, err = currI.getImageMetadata(); err != nil {
		myutils.Logger.Error("parse image metadata of image", currI.name, "failed with:", err.Error())
		return err
	}

	return nil
}

// getRepositoryMetadata gets repository metadata from local MongoDB,
// if repository not maintained in MongoDB or disconnected from MongoDB,
// try to get metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getRepositoryMetadata() (rMeta *myutils.Repository, err error) {
	// 数据库在线，尝试从数据库读取
	if myutils.GlobalDBClient.MongoFlag {
		if rMeta, err = myutils.GlobalDBClient.Mongo.FindRepositoryByName(currI.namespace, currI.repositoryName); err != nil {
			// 数据库中不存在，从API获取metadata
			rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repositoryName)
			if err != nil {
				return
			}

			// API结果正常，存入数据库，存数据库过程的error不需要返回
			if e := myutils.GlobalDBClient.Mongo.UpdateRepository(rMeta); e != nil {
				myutils.Logger.Error("update metadata of repository", currI.namespace, currI.repositoryName, "failed with:", err.Error())
			}
		} else {
			// 数据库获取正常，直接返回
			return
		}
	} else {
		// 数据库不在线，从API获取metadata并返回
		rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repositoryName)
		return
	}
	return
}

// getTagMetadata gets tag metadata from local MongoDB, if tag not maintained
// in MongoDB or disconnected from MongoDB, try to get metadata from Docker
// Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getTagMetadata() (tMeta *myutils.Tag, err error) {
	// 数据库在线，尝试从数据库读取
	if myutils.GlobalDBClient.MongoFlag {
		if tMeta, err = myutils.GlobalDBClient.Mongo.FindTagByName(currI.namespace, currI.repositoryName, currI.tagName); err != nil {
			// 数据库中不存在，从API获取metadata
			tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repositoryName, currI.tagName)
			if err != nil {
				return
			}

			// API结果正常，存入数据库，存数据库过程的error不需要返回
			if e := myutils.GlobalDBClient.Mongo.UpdateTag(tMeta); e != nil {
				myutils.Logger.Error("update metadata of tag", currI.namespace, currI.repositoryName, currI.tagName, "failed with:", err.Error())
			}
		} else {
			// 数据库获取正常，直接返回
			return
		}
	} else {
		// 数据库不在线，从API获取metadata并返回
		tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repositoryName, currI.tagName)
		return
	}
	return
}

// getImageMetadata gets image metadata from local MongoDB, if image not
// maintained in MongoDB or disconnected from MongoDB, try to get
// metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getImageMetadata() (*myutils.Image, error) {
	return nil
}

// parseConfigurationFromDockerEnv tries to inspect image from local env, with results
// stored to currI.Configuration, formatted like `docker image inspect`.
//
// returns:
//
//	bool: whether image has been stored in local Docker env.
func (currI *CurrentImage) parseConfigurationFromDockerEnv() error {
	// 从本地inspect读取镜像配置信息
	if conf, _, err := currI.dockerClient.ImageInspectWithRaw(context.TODO(), currI.name); err != nil {
		return err
	} else {
		currI.configuration = &conf
	}

	// TODO: 解析镜像的配置信息

	return nil
}
