package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-redis/cache"
)

const (
	// Badge templates to generate badges
	curveBadge = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="118" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="{{.Color}}" d="M61 0h57v20H61z"/><path fill="url(#b)" d="M0 0h118v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="315" y="140" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="885" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="470">{{.Status}}</text><text x="{{.StatusX}}" y="140" transform="scale(.1)" textLength="470">{{.Status}}</text></g> </svg>`
	flatBadge  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="{{.Color}}" d="M61 0h57v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="{{.StatusX}}" y="140" transform="scale(.1)" textLength="">{{.Status}}</text></g> </svg>`

	// errorBadge is the SVG badge returned when coverage badge could not be returned
	errorBadgeCurve = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="100" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#e05d44" d="M61 0h39v20H61z"/><path fill="url(#b)" d="M0 0h100v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="795" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="290">failed</text><text x="795" y="140" transform="scale(.1)" textLength="290">failed</text></g> </svg>`
	errorBadgeFlat  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#e05d44" d="M61 0h39v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="795" y="140" transform="scale(.1)" textLength="290">failed</text></g> </svg>`

	// progressBadge is the SVG badge returned when coverage run is in progress
	progressBadgeCurve = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="118" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="118" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#a4a61d" d="M61 0h57v20H61z"/><path fill="url(#b)" d="M0 0h118v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="885" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="470">progress</text><text x="885" y="140" transform="scale(.1)" textLength="470">progress</text></g> </svg>`
	progressBadgeFlat  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="118" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#a4a61d" d="M61 0h57v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="885" y="140" transform="scale(.1)" textLength="470">progress</text></g> </svg>`

	// queuedBadge is the SVG badge returned when a cover run request is queued
	queuedBadgeCurve = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="114" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="114" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#9f9f9f" d="M61 0h53v20H61z"/><path fill="url(#b)" d="M0 0h114v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="865" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="430">inqueue</text><text x="865" y="140" transform="scale(.1)" textLength="430">inqueue</text></g> </svg>`
	queuedBadgeFlat  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="114" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#9f9f9f" d="M61 0h53v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="865" y="140" transform="scale(.1)" textLength="430">inqueue</text></g> </svg>`
)

var (
	curveBadgeTmpl *template.Template
	flatBadgeTmpl  *template.Template
)

func init() {
	tpl := template.New("")
	curveBadgeTmpl, _ = tpl.Parse(curveBadge)
	flatBadgeTmpl, _ = tpl.Parse(flatBadge)
}

type badge struct {
	Status  string
	StatusX int
	Label   string
	Color   string
	Width   int
}

// getBadge gets the badge from img.shields.io and return as []byte
func getBadge(color, style, percent string) string {
	cacheKey := fmt.Sprintf("%s-%s-%s", color, style, percent)
	str := ""
	err := redisCodec.Get(cacheKey, &str)
	if err == nil {
		return str
	}
	errLogger.Println(err)

	imgURL := fmt.Sprintf("https://img.shields.io/badge/cover.run-%s25-%s.svg?style=%s", percent, color, style)

	resp, err := httpClient.Get(imgURL)
	if err != nil {
		errLogger.Println(err)
		return ""
	}
	defer resp.Body.Close()
	svg, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		errLogger.Println(err)
	}
	err = redisCodec.Set(&cache.Item{
		Key:        cacheKey,
		Object:     string(svg),
		Expiration: -1,
	})
	if err != nil {
		errLogger.Println(err)
	}
	return string(svg)
}

// getBadgeNew is a function which will generate SVG rather than fetch from img.shield.io
func getBadgeNew(color, style, status string) string {
	const label = "cover.run"
	buf := new(bytes.Buffer)
	switch strings.ToLower(color) {
	case "red":
		{
			color = "#d6604a"
		}
	case "green":
		{
			color = "#96c40f"
		}
	case "yellow":
		{
			color = "#d6ae22"
		}
	case "yellowgreen":
		{
			color = "#a4a61d"
		}
	case "lightgrey":
		{
			color = "#9a9a9a"
		}
	}

	b := &badge{
		Label:  label,
		Status: status,
		Color:  color,
	}
	b.StatusX = 725 + (len(b.Status) * 20)
	b.Width = 61 + (len(b.Status) * 9)

	switch style {
	case "flat", "curve", "flat-curve":
		{
			curveBadgeTmpl.Execute(buf, b)
		}
	default:
		{
			flatBadgeTmpl.Execute(buf, b)
		}
	}
	return buf.String()
}

// coverageBadge returns the SVG badge after computing the coverage
func coverageBadge(repo, tag, style string) (string, error) {
	obj, err := repoCover(repo, tag)
	if err != nil {
		if err == ErrQueueFull {
			if style == "flat-square" {
				return queuedBadgeFlat, nil
			}
			return queuedBadgeCurve, nil
		}

		if err == ErrCovInPrgrs {
			if style == "flat-square" {
				return progressBadgeFlat, nil
			}
			return progressBadgeCurve, nil
		}

		errLogger.Println(err)
		if style == "flat-square" {
			return errorBadgeFlat, err
		}
		return errorBadgeCurve, err
	}

	cover, _ := strconv.ParseFloat(strings.Replace(obj.Cover, "%", "", -1), 64)
	var color string
	if cover >= 70 {
		color = "green"
	} else if cover >= 45 {
		color = "yellow"
	} else {
		color = "red"
	}

	return getBadge(color, style, obj.Cover), nil
}
