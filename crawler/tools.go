package crawler

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
	if l < 2 {
		return ""
	}
	if flg {
		if curr[l-1] == 'z' {
			return GenerateNextKeyword(curr[:l-1], true)
		} else {
			return curr[:l-1] + string(LegalRuneList[LegalRuneMap[curr[l-1]]+1])
		}
	} else {
		return curr + string(LegalRuneList[0])
	}
}

// GetHTTPSProxy 按策略返回一个代理地址
func GetHTTPSProxy() string {
	return GetHTTPSProxyRemote()
}

// GetHTTPSProxyRemote 从远程API返回一个新的代理地址
func GetHTTPSProxyRemote() string {
	return ""
}

// GetHTTPSProxyLocal 从本地proxy pool随机返回一个代理地址
func GetHTTPSProxyLocal() string {
	return ""
}

// UpdateProxies 更新本地的proxy pool
func UpdateProxies() {
	return
}
