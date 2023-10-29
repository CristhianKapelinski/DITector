package myutils

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// configHTTPProxy configures http and https proxy.
func configHTTPProxy() {
	os.Setenv("http_proxy", "127.0.0.1:7890")
	os.Setenv("https_proxy", "127.0.0.1:7890")
}

// configTLSConfig configures not to verify CA for https protocol.
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return rMeta, err
	}

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
