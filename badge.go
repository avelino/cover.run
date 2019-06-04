package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"text/template"
)

var (
	curvedBadgeTmpl *template.Template
	flatBadgeTmpl   *template.Template
)

func init() {
	curvedBadge, err := ioutil.ReadFile("./templates/badges/curved.tmpl")
	if err != nil {
		panic(fmt.Sprintf("curved badge template not found: %v", err))
	}

	flatBadge, err := ioutil.ReadFile("./templates/badges/flat.tmpl")
	if err != nil {
		panic(fmt.Sprintf("flat badge template not found: %v", err))
	}

	tpl := template.New("")
	curvedBadgeTmpl, _ = tpl.Parse(string(curvedBadge))
	tpl2 := template.New("")
	flatBadgeTmpl, _ = tpl2.Parse(string(flatBadge))

}

type badge struct {
	Status  string
	StatusX int
	Label   string
	Color   string
	Width   int
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
		_ = curvedBadgeTmpl.Execute(buf, b)
	default:
		_ = flatBadgeTmpl.Execute(buf, b)
	}

	return buf.String()
}

// coverageBadge returns the SVG badge after computing the coverage
func coverageBadge(repo, tag, style string) string {
	obj, err := repoCover(repo, tag)
	if err != nil {
		if err == ErrQueued {
			return getBadge("lightgrey", style, "queued")
		} else if err == ErrCovInPrgrs {
			return getBadge("yellowgreen", style, "testing")
		}

		errLogger.Println(err)
	}

	var color = "red"

	percent, err := strconv.ParseFloat(strings.Replace(obj.Cover, "%", "", -1), 64)
	if err != nil {
		return getBadge(color, style, "error")
	}

	if percent >= 70 {
		color = "green"
	} else if percent >= 45 {
		color = "yellow"
	}

	status := strconv.FormatFloat(math.Round(percent*100)/100, 'f', -1, 64)
	return getBadge(color, style, status+"%")
}
