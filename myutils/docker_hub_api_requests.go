package myutils

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// configDefaultHTTPProxy configures http and https proxy.
func configDefaultHTTPProxy(httpProxy, httpsProxy string) {
	if httpProxy != "" {
		os.Setenv("http_proxy", httpProxy)
	}
	if httpsProxy != "" {
		os.Setenv("https_proxy", httpsProxy)
	}
}

// configTLSConfig configures not to verify CA for https protocol.
func configTLSConfig() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

// ReqRepoMetadata gets repository metadata by calling Docker Hub API.
func ReqRepoMetadata(namespace, name string) (*Repository, error) {
	rMeta := new(Repository)

	url := GetRepositoryMetadataURL(namespace, name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, rMeta)
	if err != nil {
		return nil, err
	}

	// 处理404
	if rMeta.Name == "" {
		return nil, fmt.Errorf("docker hub resp 404 to repo %s/%s", namespace, name)
	}

	return rMeta, nil
}

// ReqTagMetadata gets tag metadata by calling Docker Hub API.
func ReqTagMetadata(repoNamespace, repoName, name string) (*Tag, error) {
	tMeta := new(Tag)

	url := GetTagMetadataURL(repoNamespace, repoName, name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, tMeta)
	if err != nil {
		return nil, err
	}

	// 处理404
	if tMeta.Name == "" {
		return nil, fmt.Errorf("docker hub resp 404 to tag %s/%s:%s", repoNamespace, repoName, name)
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
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &isMeta)
	if err != nil {
		return nil, err
	}

	if len(isMeta) == 0 {
		return nil, fmt.Errorf("docker hub resp 404 to images %s/%s:%s", repoNamespace, repoName, name)
	}

	return isMeta, err
}
