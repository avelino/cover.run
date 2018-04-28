package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: time.Second * 20,
}

// getBadge gets the badge from img.shields.io and return as []byte
func getBadge(color, style, percent string) ([]byte, error) {
	imgURL := fmt.Sprintf("https://img.shields.io/badge/cover.run-%s25-%s.svg?style=%s", percent, color, style)

	resp, err := httpClient.Get(imgURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(io.LimitReader(resp.Body, 1024))
}
