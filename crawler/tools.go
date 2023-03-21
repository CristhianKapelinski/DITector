package crawler

// LegalRuneList 作为生成时参考的字符表
var LegalRuneList = [...]rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p',
	'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', '-', '_'}

// LegalRuneMap 用于索引rune在LegalRuneList中的下标
var LegalRuneMap = map[uint8]int{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
	'a': 10, 'b': 11, 'c': 12, 'd': 13, 'e': 14, 'f': 15, 'g': 16, 'h': 17, 'i': 18,
	'j': 19, 'k': 20, 'l': 21, 'm': 22, 'n': 23, 'o': 24, 'p': 25, 'q': 26, 'r': 27,
	's': 28, 't': 29, 'u': 30, 'v': 31, 'w': 32, 'x': 33, 'y': 34, 'z': 35,
	'-': 36, '_': 37,
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
		if curr[l-1] == '_' {
			return GenerateNextKeyword(curr[:l-1], true)
		} else {
			return curr[:l-1] + string(LegalRuneList[LegalRuneMap[curr[l-1]]+1])
		}
	} else {
		return curr + string(LegalRuneList[0])
	}
}
