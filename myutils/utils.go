package myutils

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// CalSha256 对字符串计算sha256，并返回string
func CalSha256(s string) string {
	tmpHash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(tmpHash[:])
}

// StrLegalForMongo check whether string s is
func StrLegalForMongo(s string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9:]*$", s)
	return match
}
