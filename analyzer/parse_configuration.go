package analyzer

import (
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"os"
	"path"
	"regexp"
	"strings"
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
	manifestData, err := os.ReadFile(path.Join(currI.imgFilepath, currI.manifest.Config))
	if err != nil {
		return err
	}

	if err = json.Unmarshal(manifestData, currI.configuration); err != nil {
		return err
	}

	// 根据配置具体调整平台信息
	currI.architecture, currI.variant = currI.configuration.Architecture, currI.configuration.Variant
	currI.os, currI.osVersion = currI.configuration.Os, currI.configuration.OsVersion

	// 解析容器默认启动命令
	currI.defaultCmd.entrypoint = strings.TrimSpace(strings.Join(currI.configuration.Config.Entrypoint, " "))
	currI.defaultCmd.cmd = strings.TrimSpace(strings.Join(currI.configuration.Config.Cmd, " "))
	if currI.defaultCmd.entrypoint != "" {
		currI.defaultCmd.fullCmd = strings.Join([]string{currI.defaultCmd.entrypoint, currI.defaultCmd.cmd}, " ")
	} else {
		currI.defaultCmd.fullCmd = currI.defaultCmd.cmd
	}
	// 解析容器默认执行文件路径
	stripBinShR, _ := regexp.Compile(`^(?:/bin/sh\s+-c\s+)?(.*)`)
	defaultExecFileR, _ := regexp.Compile(`^(?:python\s+|./)?(\S+)`)
	cmd := currI.defaultCmd.fullCmd
	if strings.HasPrefix(cmd, "/bin/sh -c") {
		ms := stripBinShR.FindStringSubmatch(cmd)
		if len(ms) > 1 {
			cmd = ms[1]
		}
	}
	matches := defaultExecFileR.FindStringSubmatch(cmd)
	if len(matches) > 1 {
		defaultExecFile := matches[1]
		if strings.HasPrefix(defaultExecFile, "/") {
			currI.defaultExecFile = append(currI.defaultExecFile, defaultExecFile)
		} else {
			workdir := "/"
			if currI.configuration.Config.WorkingDir != "" {
				workdir = currI.configuration.Config.WorkingDir
			}
			currI.defaultExecFile = append(currI.defaultExecFile, path.Join(workdir, defaultExecFile))
		}
	}

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
