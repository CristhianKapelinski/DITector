package main

import (
	"crawler"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// done 用于标识整个爬虫结束
var done = make(chan struct{})

func main() {
	//// 递归[]string{"00"-"zz"}, 不停尝试直到RepoList.Count < 9500, 只需要制定一个轮换规则, 记录当前状态即可
	//
	//// 当找到关键字使RepoList.Count < 9000时，遍历每一页，爬取仓库信息
	//go crawler.ScrapeRegRepoListRecursive("mongo", "community")
	//
	//// 处理Repo list，对每个Repo递归找Tag
	//time.Sleep(time.Second * 5)
	//for r := range crawler.ChannelRegRepoList {
	//	fmt.Println(r)
	//}

	//var Repo crawler.Repository__
	//c := crawler.GetRepoMetadataCollector(Repo)
	//c.Visit(crawler.GetRepoMetaURL("library", "mongo"))
	//var TagR crawler.TagReceiver__
	//c2 := crawler.GetRepoTagsCollector(&TagR)
	//c2.Visit(crawler.GetRepoTagsURL("library", "mongo", "1", "4"))
	//fmt.Println(TagR)
	//time.Sleep(time.Second * 3)
	//c3 := crawler.GetImageHistoryCollector(&TagR.Results[0].Archs)
	//c3.Visit(crawler.GetImageHistoryURL("library", "mongo", "latest"))
	//fmt.Println(TagR)

	//fmt.Println(crawler.GetNamespaceURL("aa281916", "1", "4"))
	//fmt.Println(crawler.GetRepoMetaURL("aa281916", "getting-started"))
	//fmt.Println(crawler.GetRepoTagsURL("aa281916", "getting-started", "1", "4"))
	//fmt.Println(crawler.GetImageMetaURL("aa281916", "getting-started", "latest"))
	//fmt.Println(crawler.GetImageHistoryURL("aa281916", "getting-started", "latest"))
	// 访问地址
	//for _, i := range []string{"1"} {
	//	c.Visit(strings.Replace(RegURLTemplate, "{PAGE}", i, 1))
	//}
	//c := crawler.GetDockerHubCollector()
	//fmt.Println(c)

	//sem := semaphore.NewWeighted(3)
	//var wg sync.WaitGroup
	//ctx := context.Background()
	//for i := 0; i < 10; i++ {
	//	sem.Acquire(ctx, 1)
	//	wg.Add(1)
	//	go func(j int) {
	//		time.Sleep(3 * time.Second)
	//		fmt.Println("From: ", j)
	//		defer func() {
	//			sem.Release(1)
	//			wg.Done()
	//		}()
	//	}(i)
	//}
	//wg.Wait()
	//go func() { time.Sleep(time.Second * 3); done <- struct{}{} }()
	//// 退出程序
	//<-done
	rawjson := `[
    {
        "digest": "sha256:be8ec4e48d7f24a9a1c01063e5dfabb092c2c1ec73e125113848553c9b07eb8c",
        "size": 45838270,
        "instruction": "ADD file:8eef54430e581236e6d529a7d09df648f43c840e889d9ae132e5ed25d7bd2b88 in / "
    },
    {
        "digest": "sha256:33b8b485aff0509bb0fa67dff6a2aa82e9b7b17e5ef28c1673467ec83edb945d",
        "size": 849,
        "instruction": "/bin/sh -c set -xe \t\t&& echo '#!/bin/sh' > /usr/sbin/policy-rc.d \t&& echo 'exit 101' >> /usr/sbin/policy-rc.d \t&& chmod +x /usr/sbin/policy-rc.d \t\t&& dpkg-divert --local --rename --add /sbin/initctl \t&& cp -a /usr/sbin/policy-rc.d /sbin/initctl \t&& sed -i 's/^exit.*/exit 0/' /sbin/initctl \t\t&& echo 'force-unsafe-io' > /etc/dpkg/dpkg.cfg.d/docker-apt-speedup \t\t&& echo 'DPkg::Post-Invoke { \"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\"; };' > /etc/apt/apt.conf.d/docker-clean \t&& echo 'APT::Update::Post-Invoke { \"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true\"; };' >> /etc/apt/apt.conf.d/docker-clean \t&& echo 'Dir::Cache::pkgcache \"\"; Dir::Cache::srcpkgcache \"\";' >> /etc/apt/apt.conf.d/docker-clean \t\t&& echo 'Acquire::Languages \"none\";' > /etc/apt/apt.conf.d/docker-no-languages \t\t&& echo 'Acquire::GzipIndexes \"true\"; Acquire::CompressionTypes::Order:: \"gz\";' > /etc/apt/apt.conf.d/docker-gzip-indexes \t\t&& echo 'Apt::AutoRemove::SuggestsImportant \"false\";' > /etc/apt/apt.conf.d/docker-autoremove-suggests"
    },
    {
        "digest": "sha256:d887158cc58cbfc3d03cefd5c0b15175fae66ffbf6f28a56180c51cbb5062b8a",
        "size": 533,
        "instruction": "/bin/sh -c rm -rf /var/lib/apt/lists/*"
    },
    {
        "digest": "sha256:05895bb28c18264f614acd13e401b3c5594e12d9fe90d7e52929d3e810e11e97",
        "size": 167,
        "instruction": "/bin/sh -c mkdir -p /run/systemd && echo 'docker' > /run/systemd/container"
    },
    {
        "size": 0,
        "instruction": " CMD [\"/bin/bash\"]"
    },
    {
        "size": 0,
        "instruction": "LABEL maintainer=NVIDIA CORPORATION <cudatools@nvidia.com>"
    },
    {
        "digest": "sha256:7cc9fc34ed5eb4e3e43e49921f2364417925f02459da0c8310a2f0c267e9422b",
        "size": 6845165,
        "instruction": "RUN /bin/sh -c apt-get update && apt-get install -y --no-install-recommends     ca-certificates apt-transport-https gnupg-curl &&     NVIDIA_GPGKEY_SUM=d1be581509378368edeec8c1eb2958702feedf3bc3d17011adbf24efacce4ab5 &&     NVIDIA_GPGKEY_FPR=ae09fe4bbd223a84b2ccfce3f60f4b3d7fa2af80 &&     apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64/7fa2af80.pub &&     apt-key adv --export --no-emit-version -a $NVIDIA_GPGKEY_FPR | tail -n +5 > cudasign.pub &&     echo \"$NVIDIA_GPGKEY_SUM  cudasign.pub\" | sha256sum -c --strict - && rm cudasign.pub &&     echo \"deb https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64 /\" > /etc/apt/sources.list.d/cuda.list &&     echo \"deb https://developer.download.nvidia.com/compute/machine-learning/repos/ubuntu1604/x86_64 /\" > /etc/apt/sources.list.d/nvidia-ml.list &&     apt-get purge --auto-remove -y gnupg-curl     && rm -rf /var/lib/apt/lists/* # buildkit"
    },
    {
        "size": 0,
        "instruction": "ENV CUDA_VERSION=10.2.89"
    },
    {
        "size": 0,
        "instruction": "ENV CUDA_PKG_VERSION=10-2=10.2.89-1"
    },
    {
        "digest": "sha256:c001a8e57e734cae91c9e5d6d34962156e29e8374ced2a03d23eadab41a36494",
        "size": 9541528,
        "instruction": "RUN /bin/sh -c apt-get update && apt-get install -y --no-install-recommends     cuda-cudart-$CUDA_PKG_VERSION     cuda-compat-10-2     && ln -s cuda-10.2 /usr/local/cuda &&     rm -rf /var/lib/apt/lists/* # buildkit"
    },
    {
        "digest": "sha256:993a4be22922eb4aee626cbbc9c1a20b9e8e77e68e04bfedd9ce5018cf828899",
        "size": 188,
        "instruction": "RUN /bin/sh -c echo \"/usr/local/nvidia/lib\" >> /etc/ld.so.conf.d/nvidia.conf &&     echo \"/usr/local/nvidia/lib64\" >> /etc/ld.so.conf.d/nvidia.conf # buildkit"
    },
    {
        "size": 0,
        "instruction": "ENV PATH=/usr/local/nvidia/bin:/usr/local/cuda/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    },
    {
        "size": 0,
        "instruction": "ENV LD_LIBRARY_PATH=/usr/local/nvidia/lib:/usr/local/nvidia/lib64"
    },
    {
        "size": 0,
        "instruction": "ENV NVIDIA_VISIBLE_DEVICES=all"
    },
    {
        "size": 0,
        "instruction": "ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility"
    },
    {
        "size": 0,
        "instruction": "ENV NVIDIA_REQUIRE_CUDA=cuda>=10.2 brand=tesla,driver>=396,driver<397 brand=tesla,driver>=410,driver<411 brand=tesla,driver>=418,driver<419 brand=tesla,driver>=440,driver<441"
    },
    {
        "size": 0,
        "instruction": "LABEL maintainer=NVIDIA CORPORATION <cudatools@nvidia.com>"
    },
    {
        "size": 0,
        "instruction": "ENV NCCL_VERSION=2.7.8"
    },
    {
        "digest": "sha256:0e6f6f74a34bb5b6bfb1fda645a2132c684b282648d1925c949d09f434885868",
        "size": 710553123,
        "instruction": "RUN /bin/sh -c apt-get update && apt-get install -y --no-install-recommends     cuda-libraries-$CUDA_PKG_VERSION     cuda-npp-$CUDA_PKG_VERSION     cuda-nvtx-$CUDA_PKG_VERSION     libcublas10=10.2.2.89-1     libnccl2=$NCCL_VERSION-1+cuda10.2     && apt-mark hold libnccl2     && rm -rf /var/lib/apt/lists/* # buildkit"
    },
    {
        "size": 0,
        "instruction": " ARG AMDGPU_VERSION"
    },
    {
        "size": 0,
        "instruction": " ARG BUILD_DATE"
    },
    {
        "size": 0,
        "instruction": " ARG CUDA_UBUNTU_VERSION"
    },
    {
        "size": 0,
        "instruction": " ARG CUDA_VERSION"
    },
    {
        "size": 0,
        "instruction": " ARG GIT_BRANCH"
    },
    {
        "size": 0,
        "instruction": " ARG VCS_REF"
    },
    {
        "size": 0,
        "instruction": " LABEL org.label-schema.build-date=2021-01-05T22:03:21Z org.label-schema.name=xmrig org.label-schema.description=xmrig CUDA/AMD org.label-schema.url=https://github.com/patsissons/xmrig-docker/blob/master/README.md org.label-schema.vcs-url=https://github.com/patsissons/xmrig-docker org.label-schema.vcs-ref=736dff8503dab6eba7ecc6df3ab8bfb9032229ef org.label-schema.version=v6.7.0-ubuntu16.04-cuda10.2.89-amd17.40-514569"
    },
    {
        "size": 0,
        "instruction": " ENV DEBIAN_FRONTEND=noninteractive LD_LIBRARY_PATH=/usr/local/lib:/usr/local/nvidia/lib:/usr/local/nvidia/lib64"
    },
    {
        "size": 0,
        "instruction": " ENV AMDGPU_DRIVER_NAME=amdgpu-pro-17.40-514569"
    },
    {
        "size": 0,
        "instruction": " ENV AMDGPU_DRIVER_URI=https://www2.ati.com/drivers/linux/ubuntu/amdgpu-pro-17.40-514569.tar.xz"
    },
    {
        "size": 0,
        "instruction": " ENV PACKAGE_DEPS=ca-certificates libhwloc5 libmicrohttpd10 libssl1.0.0 libuv1 wget xz-utils"
    },
    {
        "digest": "sha256:f4ede7eb5a93d4d9208a371769dcf31c923a01b90b2892034479392359464321",
        "size": 315065752,
        "instruction": "|5 AMDGPU_VERSION=17.40-514569 BUILD_DATE=2021-01-05T22:03:21Z CUDA_UBUNTU_VERSION=16.04 GIT_BRANCH=v6.7.0 VCS_REF=736dff8503dab6eba7ecc6df3ab8bfb9032229ef /bin/sh -c set -x   && dpkg --add-architecture i386   && apt-get update -qq   && apt-get install -qq --no-install-recommends -y ${PACKAGE_DEPS}   && wget -q --show-progress --progress=bar:force:noscroll --referer https://support.amd.com ${AMDGPU_DRIVER_URI}   && tar -xvf ${AMDGPU_DRIVER_NAME}.tar.xz   && SUDO_FORCE_REMOVE=yes apt-get -y remove --purge wget xz-utils   && rm -f ${AMDGPU_DRIVER_NAME}.tar.xz   && chmod +x ./${AMDGPU_DRIVER_NAME}/amdgpu-pro-install   && ./${AMDGPU_DRIVER_NAME}/amdgpu-pro-install -y   && rm -rf ${AMDGPU_DRIVER_NAME}   && rm -rf /var/opt/amdgpu-pro-local   && apt-get -qq -y autoremove   && apt-get -qq -y clean autoclean   && rm -rf /var/lib/{apt,dpkg,cache,log}"
    },
    {
        "digest": "sha256:d5b6cb688af48f999229a607ffab28bc81e6172905af19a16c6bbb436b360556",
        "size": 1737088,
        "instruction": "COPY file:98f0dded1d752615fe89cd5cabddda2f6f911ce83c4df5d59128e134433723df in /usr/local/bin/ "
    },
    {
        "digest": "sha256:d7c77e2e105b8c914cfa26c5f098e879b780d22b0950b4502e8cac645582d755",
        "size": 11342672,
        "instruction": "COPY file:05fda156ea046e1a13580b19753554bb661f3b94e3db0e3059c5067571dda14b in /usr/local/lib "
    },
    {
        "digest": "sha256:2a811bc90a839109fa1455a5287dbc2792745d618561fb0c51ddc64c22f07148",
        "size": 93,
        "instruction": "WORKDIR /config"
    },
    {
        "size": 0,
        "instruction": " VOLUME [/config]"
    },
    {
        "size": 0,
        "instruction": " ENTRYPOINT [\"/usr/local/bin/xmrig\"]"
    },
    {
        "size": 0,
        "instruction": " CMD [\"--help\"]"
    }
]`
	var ls []crawler.Layer__

	err := json.Unmarshal([]byte(rawjson), &ls)
	if err != nil {
		log.Fatalln(err)
	}

	b, _ := json.Marshal(ls)
	fmt.Println(string(b))
	time.Sleep(time.Second)
}
