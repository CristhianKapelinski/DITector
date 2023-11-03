package analyzer

import (
	"fmt"
	"myutils"
)

// parseMetadata loads metadata of repository
func (currI *CurrentImage) parseMetadata(partial bool) (err error) {
	if currI.metadata.repositoryMetadata, err = currI.getRepositoryMetadata(); err != nil {
		myutils.Logger.Error("parse repository metadata of image", currI.name, "failed with:", err.Error())
		return err
	}

	if currI.metadata.tagMetadata, err = currI.getTagMetadata(); err != nil {
		myutils.Logger.Error("parse tag metadata of image", currI.name, "failed with:", err.Error())
		return err
	}

	if partial {
		if currI.metadata.imageMetadata, err = currI.getImageMetadata(); err != nil {
			myutils.Logger.Error("parse image metadata of image", currI.name, "failed with:", err.Error())
			return err
		}
	}

	return nil
}

// getRepositoryMetadata gets repository metadata from local MongoDB,
// if repository not maintained in MongoDB or disconnected from MongoDB,
// try to get metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getRepositoryMetadata() (rMeta *myutils.Repository, err error) {
	// 数据库在线，尝试从数据库读取
	if myutils.GlobalDBClient.MongoFlag {
		if rMeta, err = myutils.GlobalDBClient.Mongo.FindRepositoryByName(currI.namespace, currI.repoName); err != nil {
			// 数据库中不存在，从API获取metadata
			rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repoName)
			if err != nil {
				return
			}

			// API结果正常，存入数据库，存数据库过程的error不需要返回
			if e := myutils.GlobalDBClient.Mongo.UpdateRepository(rMeta); e != nil {
				myutils.Logger.Error("update metadata of repository", currI.namespace, currI.repoName, "failed with:", e.Error())
			}
		} else {
			// 数据库获取正常，直接返回
			return
		}
	} else {
		fmt.Println("mongo not online, getting repository metadata from API")
		// 数据库不在线，从API获取metadata并返回
		rMeta, err = myutils.ReqRepoMetadata(currI.namespace, currI.repoName)
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
		if tMeta, err = myutils.GlobalDBClient.Mongo.FindTagByName(currI.namespace, currI.repoName, currI.tagName); err != nil {
			// 数据库中不存在，从API获取metadata
			tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repoName, currI.tagName)
			if err != nil {
				return
			}

			// API结果正常，存入数据库，存数据库过程的error不需要返回
			if e := myutils.GlobalDBClient.Mongo.UpdateTag(tMeta); e != nil {
				myutils.Logger.Error("update metadata of tag", currI.namespace, currI.repoName, currI.tagName, "failed with:", e.Error())
			}
		} else {
			// 数据库获取正常，直接返回
			return
		}
	} else {
		fmt.Println("mongo not online, getting tag metadata from API")
		// 数据库不在线，从API获取metadata并返回
		tMeta, err = myutils.ReqTagMetadata(currI.namespace, currI.repoName, currI.tagName)
		return
	}
	return
}

// getImageMetadata gets image metadata from local MongoDB, if image not
// maintained in MongoDB or disconnected from MongoDB, try to get
// metadata from Docker Hub API and store metadata to MongoDB.
func (currI *CurrentImage) getImageMetadata() (iMeta *myutils.Image, err error) {
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

	// 数据库在线，尝试从数据库读取
	if myutils.GlobalDBClient.MongoFlag {
		if iMeta, err = myutils.GlobalDBClient.Mongo.FindImageByDigest(currI.digest); err != nil {
			// 数据库中不存在，从API获取images metadata
			var isMeta []*myutils.Image
			isMeta, err = myutils.ReqImagesMetadata(currI.namespace, currI.repoName, currI.tagName)
			if err != nil {
				return
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
			if e := myutils.GlobalDBClient.Mongo.UpdateImage(iMeta); e != nil {
				myutils.Logger.Error("update metadata of image", currI.digest, "failed with:", e.Error())
			}
		} else {
			// 数据库获取正常，直接返回
			return
		}
	} else {
		fmt.Println("mongo not online, getting image metadata from API")
		// 数据库不在线，从API获取metadata并返回
		var isMeta []*myutils.Image
		isMeta, err = myutils.ReqImagesMetadata(currI.namespace, currI.repoName, currI.tagName)
		if err != nil {
			return
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

		return
	}

	return
}
