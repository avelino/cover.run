package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/go-redis/cache"
)

var netTransport = &http.Transport{
	Dial: (&net.Dialer{
		Timeout: 20 * time.Second,
	}).Dial,
	TLSHandshakeTimeout: 20 * time.Second,
}

var httpClient = &http.Client{
	Timeout:   time.Second * 20,
	Transport: netTransport,
}

// getBadge gets the badge from img.shields.io and return as []byte
func getBadge(color, style, percent string) ([]byte, error) {
	imgURL := fmt.Sprintf("https://img.shields.io/badge/cover.run-%s25-%s.svg?style=%s", percent, color, style)

	req, err := http.NewRequest(http.MethodGet, imgURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// setBadgeCache will get the badge and set it inside redis as plain string (svg)
func setBadgeCache(imgName, bdgSVG string) error {
	return redisCodec.Set(&cache.Item{
		Key:    imgName,
		Object: bdgSVG,
		// Disabling expiry
		Expiration: -1,
	})
}

// getBadgeCache gets the image from redis
func getBadgeCache(imgName string) (string, error) {
	// Maximum 1KB size, 1024 bytes
	bdgBytes := ""
	err := redisCodec.Get(imgName, &bdgBytes)
	if err != nil {
		return "", err
	}
	return bdgBytes, nil
}

// serveBadge serves the SVG file with the required response headers
func serveBadge(w http.ResponseWriter, badge string) {
	w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
	w.Header().Set("Content-Encoding", "br")
	fmt.Fprint(w, badge)
}
