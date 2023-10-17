package myutils

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// CalSha256 对字符串计算sha256，并返回string
func CalSha256(s string) string {
	tmpHash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(tmpHash[:])
}

// StrLegalForRepository check whether string s is legal for repository search
func StrLegalForRepository(s string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9:\-]*$`, s)
	return match
}

// StrLegalForImage check whether string s is legal for image search
func StrLegalForImage(s string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9:]*$", s)
	return match
}

// DivideImageName TODO: 拆解镜像名称，[registry/][namespace/]repository[:tag][@digest]
func DivideImageName(name string) (registry, namespace, repository, tag, digest string) {
	parts := strings.Split(name, ":")
	// 暂时还有问题，需要修复
	repoParts := strings.Split(parts[0], "/")
	switch len(repoParts) {
	case 1:
		registry = "docker.io"
		namespace = "library"
		repository = repoParts[0]
	case 2:
		registry = "docker.io"
		namespace = repoParts[0]
		repository = repoParts[1]
	case 3:
		registry = repoParts[0]
		namespace = repoParts[1]
		repository = repoParts[2]
	}
	repository = parts[0]
	if len(parts) == 2 {
		if strings.Contains(parts[1], "@") {
			digest = parts[1]
		} else {
			tag = parts[1]
		}
	} else if len(parts) == 3 {
		namespace = parts[0]
		repository = parts[1]
		if strings.Contains(parts[2], "@") {
			digest = parts[2]
		} else {
			tag = parts[2]
		}
	}
	return
}
