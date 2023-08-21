package crawler

import "fmt"

const (
	RegURLTemplate          = `https://hub.docker.com/api/content/v1/products/search?q=%s&source=%s&page=%s&page_size=%s`
	NamespaceURLTemplate    = `https://hub.docker.com/v2/repositories/%s?page=%s&page_size=%s&ordering=last_updated`
	RepoMetaURLTemplate     = `https://hub.docker.com/v2/repositories/%s/%s/`
	RepoTagsURLTemplate     = `https://hub.docker.com/v2/repositories/%s/%s/tags/?page=%s&page_size=%s&name&ordering`
	ImageMetaURLTemplate    = `https://hub.docker.com/v2/repositories/%s/%s/tags/%s`
	ImageHistoryURLTemplate = `https://hub.docker.com/v2/repositories/%s/%s/tags/%s/images`
)

// GetRegURL 返回用于获取repository list的URL
//
// GetRegURL 要求query参数至少包含2个字符。
// 以query为关键字，获取相关repository list的JSON格式响应，最大响应结果数目为10000。
// source可选范围: official, community, publisher(可能要求url中的q换为query)。
func GetRegURL(query, source, page, size string) string {
	return fmt.Sprintf(RegURLTemplate, query, source, page, size)
}

// GetNamespaceURL 返回用于获取namespace下repository list的URL
//
// namespace为DockerHub Register下的用户名。
func GetNamespaceURL(namespace, page, size string) string {
	return fmt.Sprintf(NamespaceURLTemplate, namespace, page, size)
}

// GetRepoMetaURL 返回Repository的元数据URL
//
// 主要包括star_count, pull_count, 最近更新时间等。
func GetRepoMetaURL(namespace, repo string) string {
	return fmt.Sprintf(RepoMetaURLTemplate, namespace, repo)
}

// GetRepoTagsURL 返回Repository TagName List的URL
//
// 主要包括Tag数量，以及每个Tag的digest、最近拉取时间、最近更新时间等。
func GetRepoTagsURL(namespace, repo, page, size string) string {
	return fmt.Sprintf(RepoTagsURLTemplate, namespace, repo, page, size)
}

// GetImageMetaURL 返回指定Image(Repo:TagName)的元数据URL
//
// 内容与GetRepoTagsURL中对每个Tag单独的描述完全一致，可以略过
func GetImageMetaURL(namespace, repo, tag string) string {
	return fmt.Sprintf(ImageMetaURLTemplate, namespace, repo, tag)
}

// GetImageHistoryURL 返回指定Image(Repo:TagName)的Layer URL
//
// 主要包括镜像包含的构建信息，即各层的digest、构建命令等。
// 对于支持多种内核架构的Tag，会以列表形式记录每个架构下的构建信息。
func GetImageHistoryURL(namespace, repo, tag string) string {
	return fmt.Sprintf(ImageHistoryURLTemplate, namespace, repo, tag)
}
