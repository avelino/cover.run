package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-redis/cache"
)

const (
	// Badge templates to generate badges
	curveBadge = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="{{.Width}}" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="{{.Color}}" d="M61 0h53v20H61z"/><path fill="url(#b)" d="M0 0h114v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="315" y="140" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="{{.StatusX}}" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="">{{.Status}}</text><text x="{{.StatusX}}" y="140" transform="scale(.1)" textLength="">{{.Status}}</text></g> </svg>`
	flatBadge  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="{{.Color}}" d="M61 0h57v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">{{.Label}}</text><text x="{{.StatusX}}" y="140" transform="scale(.1)" textLength="">{{.Status}}</text></g> </svg>`
)

var (
	curveBadgeTmpl *template.Template
	flatBadgeTmpl  *template.Template
)

func init() {
	tpl := template.New("")
	curveBadgeTmpl, _ = tpl.Parse(curveBadge)
	tpl2 := template.New("")
	flatBadgeTmpl, _ = tpl2.Parse(flatBadge)
}

type badge struct {
	Status  string
	StatusX int
	Label   string
	Color   string
	Width   int
}

// getBadge gets the badge from img.shields.io and return as []byte
func getBadgeImgShield(color, style, percent string) string {
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

// getBadge is a function which will generate SVG rather than fetch from img.shield.io
func getBadge(color, style, status string) string {
	const label = "cover.run"
	buf := new(bytes.Buffer)
	switch strings.ToLower(color) {
	case "red":
		color = "#d6604a"
	case "green":
		color = "#96c40f"
	case "yellow":
		color = "#d6ae22"
	case "yellowgreen":
		color = "#a4a61d"
	default:
		color = "#9a9a9a"
	}

	b := &badge{
		Label:  label,
		Status: status,
		Color:  color,
	}

	switch len(b.Status) {
	case 1:
		b.StatusX = 725
		b.Width = 78
	case 2:
		b.StatusX = 745
		b.Width = 90
	case 3:
		b.StatusX = 775
		b.Width = 96
	case 4:
		b.StatusX = 815
		b.Width = 104
	case 5:
		b.StatusX = 835
		b.Width = 108
	case 6:
		b.StatusX = 865
		b.Width = 114
	case 7, 8, 9:
		b.StatusX = 895
		b.Width = 120

	default:
		// 50units for 7 chars
		statusWidth := (len(b.Status) * 8)
		b.StatusX = 685 + (statusWidth + 160)
		b.Width = 61 + statusWidth + 32
	}

	switch style {
	case "flat", "curve", "flat-curve":
		curveBadgeTmpl.Execute(buf, b)
	default:
		flatBadgeTmpl.Execute(buf, b)
	}

	return buf.String()
}

// coverageBadge returns the SVG badge after computing the coverage
func coverageBadge(repo, tag, style string) (string, error) {
	obj, err := repoCover(repo, tag)
	if err != nil {
		if err == ErrQueued {
			return getBadge("lightgrey", style, "queued"), nil
		}

		if err == ErrCovInPrgrs {
			return getBadge("yellowgreen", style, "testing"), nil
		}

		errLogger.Println(err)
	}

	var color = "red"

	percent, err := strconv.ParseFloat(strings.Replace(obj.Cover, "%", "", -1), 64)
	if err != nil {
		return getBadge(color, style, "error"), nil
	}

	if percent >= 70 {
		color = "green"
	} else if percent >= 45 {
		color = "yellow"
	}

	status := strconv.FormatFloat(math.Round(percent*100)/100, 'f', -1, 64)
	return getBadge(color, style, status+"%"), nil
}
