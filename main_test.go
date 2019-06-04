package main

import (
	"strings"
	"testing"
)

func TestImageSupported(t *testing.T) {
	tt := []string{"1.11", "1.10", "1.9", "1.8"}
	for _, tag := range tt {
		if !langVersionSupported("golang-" + tag) {
			t.Log(tag, " should be suported")
			t.Fail()
		}
	}

	if langVersionSupported("golang-1.7") {
		t.Log("1.7 should not be suported")
		t.Fail()
	}
}

func TestGetBadge(t *testing.T) {
	str := getBadge("red", "flat", "100%")
	if strings.Contains("<svg xmlns", str) {
		t.Log("Expected svg badge, got", str)
		t.Fail()
	}
}
