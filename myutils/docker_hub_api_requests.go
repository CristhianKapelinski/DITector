package myutils

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func configTLSConfig() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

// TODO: 尚未处理三个Req函数得到的返回结果是404的情况

// ReqRepoMetadata gets repository metadata by calling Docker Hub API.
func ReqRepoMetadata(namespace, name string) (*Repository, error) {
	rMeta := new(Repository)

	url := GetRepositoryMetadataURL(namespace, name)
	resp, err := http.Get(url)
	if err != nil {
		return rMeta, err
	}

	fmt.Println(resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rMeta, err
	}

	fmt.Println(body)
	fmt.Println(string(body))

	err = json.Unmarshal(body, rMeta)

	return rMeta, err
}

// ReqTagMetadata gets tag metadata by calling Docker Hub API.
func ReqTagMetadata(repoNamespace, repoName, name string) (*Tag, error) {
	tMeta := new(Tag)

	url := GetTagMetadataURL(repoNamespace, repoName, name)
	resp, err := http.Get(url)
	if err != nil {
		return tMeta, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tMeta, err
	}

	err = json.Unmarshal(body, tMeta)
	if err != nil {
		return tMeta, err
	}

	tMeta.RepositoryNamespace = repoNamespace
	tMeta.RepositoryName = repoName

	return tMeta, err
}

// ReqImagesMetadata gets image metadata by calling Docker Hub API.
func ReqImagesMetadata(repoNamespace, repoName, name string) ([]*Image, error) {
	isMeta := make([]*Image, 0)

	url := GetImageMetadataURL(repoNamespace, repoName, name)
	resp, err := http.Get(url)
	if err != nil {
		return isMeta, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return isMeta, err
	}

	err = json.Unmarshal(body, &isMeta)

	return isMeta, err
}
