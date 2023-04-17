package crawler

import (
	"fmt"
	"testing"
)

func TestGenerateNextKeyword(t *testing.T) {
	if GenerateNextKeyword("000", true) != "001" {
		t.Fatal("000 t -> 001")
	}
	if GenerateNextKeyword("000", false) != "000-" {
		t.Fatal("000 f -> 000-")
	}
	if GenerateNextKeyword("00z", true) != "01" {
		t.Fatal("00z t -> 01")
	}
	if GenerateNextKeyword("aazzz", true) != "ab" {
		t.Fatal("aazzz t -> ab")
	}
	if GenerateNextKeyword("aazzz", false) != "aazzz-" {
		t.Fatal("aazzz f -> aazzz-")
	}
	if GenerateNextKeyword("azzzzz", true) != "" {
		t.Fatal("azzzzz f -> ")
	}
	if GenerateNextKeyword("abc-", true) != "abc0" {
		t.Fatal("abc- t -> abc0")
	}
}

func TestKDLProxiesMaintainer(t *testing.T) {
	KDLProxiesMaintainer()
}

func TestGetHTTPSProxy(t *testing.T) {
	fmt.Println(GenerateNextKeyword("1zzzzz", true))
}
