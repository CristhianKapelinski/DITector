package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/kuaidaili/golang-sdk/api-sdk/kdl/auth"
	"github.com/kuaidaili/golang-sdk/api-sdk/kdl/client"
	"github.com/kuaidaili/golang-sdk/api-sdk/kdl/signtype"
	"log"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"
)

// LegalRuneList 作为生成时参考的字符表
var LegalRuneList = [...]rune{
	'-',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'_',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p',
	'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
}

// LegalRuneMap 用于索引rune在LegalRuneList中的下标
var LegalRuneMap = map[uint8]int{
	'-': 0,
	'0': 1, '1': 2, '2': 3, '3': 4, '4': 5, '5': 6, '6': 7, '7': 8, '8': 9, '9': 10,
	'_': 11,
	'a': 12, 'b': 13, 'c': 14, 'd': 15, 'e': 16, 'f': 17, 'g': 18, 'h': 19, 'i': 20,
	'j': 21, 'k': 22, 'l': 23, 'm': 24, 'n': 25, 'o': 26, 'p': 27, 'q': 28, 'r': 29,
	's': 30, 't': 31, 'u': 32, 'v': 33, 'w': 34, 'x': 35, 'y': 36, 'z': 37,
}

// GenerateNextKeyword 用于根据当前关键字字符串生成下一个可选的字符。
// curr: 当前在搜索字符
// flg: 当前字符count<9000为true，否则为false
func GenerateNextKeyword(curr string, flg bool) string {
	l := len(curr)
	//if l < 2 {
	//	return ""
	//}
	if flg {
		if curr[l-1] == 'z' {
			if l == 2 {
				if curr[0] != 'z' {
					return string(LegalRuneList[LegalRuneMap[curr[0]]+1]) + "-"
				} else {
					return ""
				}
			}
			return GenerateNextKeyword(curr[:l-1], true)
		} else {
			return curr[:l-1] + string(LegalRuneList[LegalRuneMap[curr[l-1]]+1])
		}
	} else {
		return curr + string(LegalRuneList[0])
	}
}

// GetHTTPSProxy 从Proxies中随机返回一个代理
func GetHTTPSProxy() string {
	// 目前用于快代理
	p := GetProxyList()
	cnt := 0
	for len(p) == 0 {
		cnt++
		fmt.Println("[WARN] Proxies Null, Waiting for update...")
		if cnt > 12 {
			fmt.Println("[ERROR] Proxies Null for a period, exit!!!")
			panic("Proxies Null")
		}
		time.Sleep(5 * time.Second)
		p = GetProxyList()
	}
	return "http://" + p[rand.Intn(len(p))]
}

// KDLProxiesMaintainer 利用快代理提供的go-sdk持续维护本地的Proxies变量
func KDLProxiesMaintainer() {
	// 读取快代理secret_id和secret_key
	secret := struct {
		Id  string `json:"secret_id"`
		Key string `json:"secret_key"`
	}{}
	_, filename, _, _ := runtime.Caller(0)
	rootdir := path.Dir(path.Dir(filename))
	secretFile := rootdir + "/crawler/secret.json"
	// 加载DockerCrawler Config
	fb, err := os.ReadFile(secretFile)
	if err != nil {
		fmt.Println("[ERROR] Failed to load ", secretFile, err)
	}
	if err = json.Unmarshal(fb, &secret); err != nil {
		fmt.Printf("[ERROR] Json failed to unmarshal %s with err: %v\n", secretFile, err)
	}

	// 创建快代理sdk要求的客户端
	kdlAuth := auth.Auth{SecretID: secret.Id, SecretKey: secret.Key}
	kdlClient := client.Client{Auth: kdlAuth}

	// 获取订单到期时间, 返回时间字符串
	//expireTime, err := kdlClient.GetOrderExpireTime(signtype.HmacSha1)
	//if err != nil {
	//	log.Println(err)
	//}
	//fmt.Println("expire time: ", expireTime)

	//设置ip白名单，参数类型为[]string
	//_, err = kdlClient.SetIPWhitelist([]string{"58.246.183.50"}, signtype.HmacSha1)
	//if err != nil {
	//	log.Fatal("[ERROR] Kuaidaili SetIPWhitelist failed with: ", err)
	//}

	// 每5秒钟检查一次Proxies.Addresses中的代理存活期，如果剩余存活时间不足10s，则发起请求更换为新的代理地址
	for {
		UpdateProxies(&kdlClient)
		time.Sleep(5 * time.Second)
	}
}

// UpdateProxies 更新本地的proxies。
// 检查本地IP有效性与存活时间，对于无效和存活时间不足10s的，更换新的一批进来
func UpdateProxies(c *client.Client) {
	// 记录总的需要更新的量
	p := GetProxyList()
	cnt := 12 - len(p) // 不为0表示正在初始化Proxies

	if cnt != 0 {
		Proxies.Valid = make(map[string]bool)
	}

	// 尝试更新直到有10个验证有效的代理
	for {
		// 不是初始化，而是日常检查代理状态
		if cnt == 0 {
			// 删除失效代理
			// 检测私密代理有效性， 返回map[string]bool, ip:true/false
			valids, err := c.CheckDpsValid(GetProxyList(), signtype.HmacSha1)
			if err != nil {
				log.Println("[ERROR] Kuaidaili CheckDpsValid failed with: ", err)
			}
			for ip, valid := range valids {
				if !valid {
					delete(Proxies.Valid, ip)
					cnt++
				}
			}

			// 删除存活时间不足10s的代理
			// 获取私密代理剩余时间(单位为秒), 返回map[string]string, ip:seconds
			seconds, err := c.GetDpsValidTime(GetProxyList(), signtype.Token)
			if err != nil {
				log.Println("[ERROR] Kuaidaili GetDpsValidTime failed with: ", err)
			}
			for ip, sec := range seconds {
				if i, _ := strconv.Atoi(sec); i < 10 {
					delete(Proxies.Valid, ip)
					cnt++
				}
			}
		}

		// 当前代理全部有效，退出
		if cnt == 0 {
			//fmt.Println("[INFO] UpdateProxies succeed!!!")
			return
		}

		// 获取新的代理填入Proxies
		// 提取私密代理, 参数有: 提取数量、鉴权方式及其他参数(放入map[string]interface{}中, 若无则传入nil)
		// (具体有哪些其他参数请参考帮助中心: "https://www.kuaidaili.com/doc/api/getdps/")
		params := map[string]interface{}{"format": "json"}
		ips, err := c.GetDps(cnt, signtype.HmacSha1, params)
		if err != nil {
			log.Println(err)
		}
		for _, ip := range ips {
			Proxies.Valid[ip] = true
		}

		cnt = 0
	}
}

// GetProxyList 返回Proxies.Valid的键列表。
// 仅对快代理部分有效
func GetProxyList() []string {
	proxies := make([]string, len(Proxies.Valid))
	i := 0
	for k, _ := range Proxies.Valid {
		proxies[i] = k
		i++
	}
	return proxies
}
