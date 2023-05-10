As its name shows, DockerCrawler implements a multi-thread crawler for metadata of all images from Docker Hub.
Metadata includes:

- Basic information, like namespace, repository, tags of the image, 
and the layers of each architecture of each tag, etc.
- 

*This project is the implementation for monitor module of Project DockerScanner.*

## Install

项目需求环境较为复杂，需要提前配置以下环境：

配置mysql数据库环境，并为mysql数据库创建新用户docker，密码docker。

由于项目涉密需求，本项目并未开源到任何平台，因此需要先配置go workspace，在将本项目的go模块注册到本地工作空间中。具体步骤如下：

- 在项目根目录`DockerCrawler`下执行：`go work init`
- 在`buildgraph`目录下执行：`go get`，再回到`DockerCrawler`下执行：`go work use buildgraph`
- 在`crawler`目录下执行：`go get`，再回到`DockerCrawler`下执行：`go work use crawler`
- 对`db`、`dockercrawler`同理，安装依赖并将其注册到go.work中

## Usage
### Basic Usage

基本使用（以下命令均在项目根目录下执行）：

爬取Docker Hub镜像仓库的社区镜像并存入mysql：

`go run dockercrawler -crawl dockerhub -format mysql`

爬取Docker Hub镜像仓库的官方镜像并存为json：

`go run dockercrawler -crawl dockerhub -official -format json`

### Configs

我们提供了两种方式（命令行、配置文件）来配置指导程序运行，两种配置支持的内容不重复。

命令行配置内容（可通过命令`go run dockercrawler -help`查看）：

- -crawl：配置要爬取的镜像仓库，目前只支持dockerhub，默认为空
- -official：是否爬取官方镜像，默认false
- -build-graph：是否要根据爬虫结果建立信息库，默认false
- -format：配置爬虫存储格式/build-graph的数据源格式，支持json、mysql，默认json


### Proxies

Docker Hub有访问频率限制：每个IP地址 180次/某时间段, DockerCrawler允许自己配置代理池来防止IP访问过快被禁止访问。

如果您已经有一个稳定的静态IP代理池，那么可以在`crawler/`路径下创建`proxyaddr.json`文件（具体路径可以通过config.json设置），按照如下格式组织内容，即可配置代理池。

```json
{
  "proxies": [
    "proxyaddr1.com",
    "proxyaddr2.com",
    "..."
  ]
}
```

我们在建设信息库时购买了[快代理](https://www.kuaidaili.com/)的私密代理（存活时间7-15分钟）作为IP池，并实现了配套的IP池更新机制。如果想了解具体更新机制或参照实现其他IP代理提供商的更新机制，详见`crawler/tools.go`中的`KDLProxiesMaintainer(), KDLUpdateProxies(), KDLGetProxyList()`。

Besides, we originally use proxies from [kuaidaili](https://www.kuaidaili.com/) and implement automatic proxy
updater to monitor the life of every proxy-ip and substitude those ips to be out-of-life with new ones.

如果您也打算使用快代理作为IP池，那么无需更新或修改其他代码。购买快代理私密代理后，在项目根目录下创建文件`secret.json`，按照如下格式组织内容，即可配置代理池。

If you decide to use kuaidaili as proxy-ip source too, just create a file named "secret.json" under the root directory
, and the content should be:

```json
{
  "secret_id": "",
  "secret_key": ""
}
```

### Other Configs

## Documents

程序设计思路参考`docs/dev.md`(中文文档，作者发电)