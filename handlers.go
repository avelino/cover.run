package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// HandlerRepoJSON returns the coverage details of a repository as JSON
func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	goversion := strings.TrimSpace(r.URL.Query().Get("tag"))
	if goversion == "" {
		goversion = DefaultTag
	}

	vars := mux.Vars(r)
	repo := strings.TrimSpace(vars["repo"])

	obj, _ := repoCover(repo, goversion)
	json.NewEncoder(w).Encode(obj)
}

// HandlerRepoSVG returns the SVG badge with coverage for a given repository
func HandlerRepoSVG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		tag = DefaultTag
	}
	repo := strings.TrimSpace(vars["repo"])

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

// HandlerBadge generates a badge with the given value
func HandlerBadge(w http.ResponseWriter, r *http.Request) {
	style := strings.TrimSpace(r.URL.Query().Get("style"))
	color := strings.TrimSpace(r.URL.Query().Get("color"))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	svg := getBadge(color, style, value)

	w.Header().Set("cache-control", "priviate, max-age=0, no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Write([]byte(svg))
}

type pageData struct {
	Versions map[string]string
}

// Handler returns the homepage
func Handler(w http.ResponseWriter, r *http.Request) {
	bind := pageData{
		Versions: langSupportedVersions,
	}

	if err := pageTmpl.Execute(w, bind); err != nil {
		errLogger.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
