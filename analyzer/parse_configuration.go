package analyzer

import (
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"os"
	"time"
)

type Configuration struct {
	Config          *container.Config `json:"config"`
	Container       string            `json:"container"`
	ContainerConfig *container.Config `json:"container_config"`
	Created         time.Time         `json:"created"`
	Architecture    string            `json:"architecture"`
	Variant         string            `json:"variant,omitempty"`
	Os              string            `json:"os"`
	OsVersion       string            `json:"os_version,omitempty"`
	RootFS          *RootFS           `json:"rootfs"`
}

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

// parseConfigurationFromFile TODO: loads image config from file <digest>.json (CurrentImage.manifest.Config).
func (currI *CurrentImage) parseConfigurationFromFile() error {
	manifestData, err := os.ReadFile(currI.manifest.Config)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(manifestData, currI.configuration); err != nil {
		return err
	}

	// 根据配置具体调整平台信息
	currI.architecture, currI.variant = currI.configuration.Architecture, currI.configuration.Variant
	currI.os, currI.osVersion = currI.configuration.Os, currI.configuration.OsVersion

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
	//if conf, _, err := currI.dockerClient.ImageInspectWithRaw(context.TODO(), currI.name); err != nil {
	//	return err
	//} else {
	//	currI.configuration = &conf
	//}

	// TODO: 解析镜像的配置信息

	return nil
}
