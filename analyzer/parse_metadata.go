package analyzer

import (
	"fmt"
	"github.com/Musso12138/docker-scan/myutils"
	"regexp"
)

var (
	extRecommendCmdRE = regexp.MustCompile(`docker\s+run.*(?:\\[\n\r].+)*.`)
)

// parseMetadata loads metadata of repository
func (currI *CurrentImage) parseMetadata(partial, fromAPI bool) error {

	if err := currI.parseRepoMetadata(fromAPI); err != nil {
		return err
	}

	if err := currI.parseTagMetadata(fromAPI); err != nil {
		return err
	}

	if partial {
		if err := currI.parseImageMetadata(fromAPI); err != nil {
			return err
		}
	}

	return nil
}

func (currI *CurrentImage) parseRepoMetadata(fromAPI bool) (err error) {
	if !currI.repoMetaFromAPI {
		if currI.metadata.repositoryMetadata, err = currI.getRepositoryMetadata(fromAPI); err != nil {
			myutils.Logger.Error("parse repository metadata of image", currI.name, "failed with:", err.Error())
			return err
		}
	}

	// 提取推荐容器启动命令
	currI.recommendedCmd = extractRecommendCmd(currI.metadata.repositoryMetadata.FullDescription)

	return nil
}

// getRepositoryMetadata gets repository metadata from local MongoDB,
// if repository not maintained in MongoDB or disconnected from MongoDB,
// try to get metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getRepositoryMetadata(fromAPI bool) (*myutils.Repository, error) {
	var rMeta *myutils.Repository
	var err error

	// 要求从API获取
	if fromAPI {
		rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repoName)
	} else {
		// 不要求从网络获取时先尝试从MongoDB获取
		if myutils.GlobalDBClient.MongoFlag {
			if rMeta, err = myutils.GlobalDBClient.Mongo.FindRepositoryByName(currI.namespace, currI.repoName); err == nil {
				return rMeta, nil
			}
		}
		// MongoDB返回err，从API获取
		rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repoName)
	}
	// 从API获取时err非空
	if err != nil {
		return nil, err
	}

	// API结果正常，存入数据库，存数据库过程的error不需要返回
	if myutils.GlobalDBClient.MongoFlag {
		currI.wg.Add(1)
		go func(repoMetadata *myutils.Repository) {
			defer currI.wg.Done()
			if e := myutils.GlobalDBClient.Mongo.UpdateRepository(repoMetadata); e != nil {
				myutils.Logger.Error("update metadata of repository", repoMetadata.Namespace, repoMetadata.Name, "failed with:", e.Error())
			}
		}(rMeta)
	}

	// 标记元数据来自API
	currI.repoMetaFromAPI = true

	return rMeta, nil
}

func (currI *CurrentImage) parseTagMetadata(fromAPI bool) (err error) {
	if !currI.tagMetaFromAPI {
		if currI.metadata.tagMetadata, err = currI.getTagMetadata(fromAPI); err != nil {
			myutils.Logger.Error("parse tag metadata of image", currI.name, "failed with:", err.Error())
			return err
		}
	}

	return nil
}

// getTagMetadata gets tag metadata from local MongoDB, if tag not maintained
// in MongoDB or disconnected from MongoDB, try to get metadata from Docker
// Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getTagMetadata(fromAPI bool) (*myutils.Tag, error) {
	var tMeta *myutils.Tag
	var err error

	if fromAPI {
		tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repoName, currI.tagName)
	} else {
		if myutils.GlobalDBClient.MongoFlag {
			if tMeta, err = myutils.GlobalDBClient.Mongo.FindTagByName(currI.namespace, currI.repoName, currI.tagName); err == nil {
				return tMeta, err
			}
		}
		tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repoName, currI.tagName)
	}

	if err != nil {
		return nil, err
	}

	if myutils.GlobalDBClient.MongoFlag {
		currI.wg.Add(1)
		go func(tagMetadata *myutils.Tag) {
			defer currI.wg.Done()
			if e := myutils.GlobalDBClient.Mongo.UpdateTag(tagMetadata); e != nil {
				myutils.Logger.Error("update metadata of tag", tagMetadata.RepositoryNamespace, tagMetadata.RepositoryName, tagMetadata.Name, "failed with:", e.Error())
			}
		}(tMeta)
	}

	// 标记元数据来自API
	currI.tagMetaFromAPI = true

	return tMeta, nil
}

func (currI *CurrentImage) parseImageMetadata(fromAPI bool) (err error) {
	if !currI.imgMetaFromAPI {
		if currI.metadata.imageMetadata, err = currI.getImageMetadata(fromAPI); err != nil {
			myutils.Logger.Error("parse image metadata of image", currI.name, "failed with:", err.Error())
			return err
		}
	}

	return nil
}

// getImageMetadata gets image metadata from local MongoDB, if image not
// maintained in MongoDB or disconnected from MongoDB, try to get
// metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getImageMetadata(fromAPI bool) (*myutils.Image, error) {
	var iMeta *myutils.Image
	var err error
	var isMeta []*myutils.Image

	// 检查是否有architecture和os记录
	if currI.architecture == "" || currI.os == "" {
		return nil, fmt.Errorf("no architecture or os parsed in current image")
	}

	// 根据arch, os匹配tag元数据中的镜像digest
	for _, iit := range currI.metadata.tagMetadata.Images {
		// 命中arch和os时覆盖digest
		if currI.architecture == iit.Architecture && currI.os == iit.OS {
			currI.digest = iit.Digest
			// 信息全部命中时直接退出
			if currI.variant == iit.Variant && currI.osVersion == iit.OSVersion {
				break
			}
		}
	}

	if currI.digest == "" {
		return nil, fmt.Errorf("no image with the same platform %s was found in tag %s metadata",
			currI.os+"/"+currI.architecture, currI.registry+"/"+currI.namespace+"/"+currI.repoName+":"+currI.tagName)
	}

	// 要求从API获取
	if fromAPI {
		isMeta, err = myutils.ReqImagesMetadata(currI.namespace, currI.repoName, currI.tagName)
	} else {
		if myutils.GlobalDBClient.MongoFlag {
			if iMeta, err = myutils.GlobalDBClient.Mongo.FindImageByDigest(currI.digest); err == nil {
				return iMeta, nil
			}
		}
		isMeta, err = myutils.ReqImagesMetadata(currI.namespace, currI.repoName, currI.tagName)
	}

	if err != nil {
		return nil, err
	}

	// 根据digest匹配对应的image metadata
	for _, iit := range isMeta {
		if currI.digest == iit.Digest {
			iMeta = iit
			break
		}
	}
	if iMeta == nil {
		return nil, fmt.Errorf("no image with the same platform %s was found in tag %s metadata",
			currI.os+"/"+currI.architecture, currI.registry+"/"+currI.namespace+"/"+currI.repoName+":"+currI.tagName)
	}

	// API结果正常，存入数据库，存数据库过程的error不需要返回
	if myutils.GlobalDBClient.MongoFlag {
		for _, imgMeta := range isMeta {
			currI.wg.Add(1)
			go func(imgMetadata *myutils.Image) {
				defer currI.wg.Done()
				if e := myutils.GlobalDBClient.Mongo.UpdateImage(imgMetadata); e != nil {
					myutils.Logger.Error("update metadata of image", imgMetadata.Digest, "failed with:", e.Error())
				}
			}(imgMeta)
		}
	}

	// 标记元数据来自API
	currI.imgMetaFromAPI = true

	return iMeta, nil
}

// extractRecommendCmd extracts container start command recommended
// by the author like `docker run` from full_description in repo metadata.
func extractRecommendCmd(desc string) []string {
	return extRecommendCmdRE.FindAllString(desc, -1)
}
