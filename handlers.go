package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
)

var (
	regGithub  = regexp.MustCompile(`(?i)^(github.com)(.*)$`)
	replaceStr = "github.com$2"
)

// HandlerRepoJSON returns the coverage details of a repository as JSON
func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		tag = DefaultTag
	}
	repo := strings.TrimSpace(vars["repo"])
	repo = regGithub.ReplaceAllString(repo, replaceStr)

	obj, err := repoCover(repo, tag)
	if err == nil || err == ErrCovInPrgrs || err == ErrQueued || err == ErrNoTest {
		if err != nil {
			obj.Cover = err.Error()
		}
		json.NewEncoder(w).Encode(obj)
		return
	}

	errLogger.Println(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// HandlerRepoSVG returns the SVG badge with coverage for a given repository
func HandlerRepoSVG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		tag = DefaultTag
	}
	repo := strings.TrimSpace(vars["repo"])
	repo = regGithub.ReplaceAllString(repo, replaceStr)

	badgeStyle := strings.TrimSpace(r.URL.Query().Get("style"))
	if badgeStyle != "flat" {
		badgeStyle = "flat-square"
	}

	svg, _ := coverageBadge(repo, tag, badgeStyle)

	w.Header().Set("cache-control", "priviate, max-age=0, no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Vary", "Accept-Encoding")

	w.Write([]byte(svg))
}

// Handler returns the homepage
func Handler(w http.ResponseWriter, r *http.Request) {
	err := pageTmpl.Execute(w, nil)
	if err != nil {
		errLogger.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
