package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// HandlerRepoJSON returns the coverage details of a repository as JSON
func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}
	obj, err := repoCover(vars["repo"], tag)
	if err == nil || err == ErrCovInPrgrs || err == ErrQueued {
		if err != nil {
			obj.Cover = err.Error()
		}
		json.NewEncoder(w).Encode(obj)
		return
	}

	if err != ErrCovInPrgrs && err != ErrQueued {
		errLogger.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandlerRepoSVG returns the SVG badge with coverage for a given repository
func HandlerRepoSVG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}

	badgeStyle := r.URL.Query().Get("style")
	if badgeStyle != "flat" {
		badgeStyle = "flat-square"
	}

	svg, _ := coverageBadge(vars["repo"], tag, badgeStyle)

	w.Header().Set("cache-control", "priviate, max-age=0, no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Write([]byte(svg))
}

// HandlerRepo has the result of a repository cover run
func HandlerRepo(w http.ResponseWriter, r *http.Request) {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		tag = DefaultTag
	}

	vars := mux.Vars(r)
	if repo == "" {
		repo = strings.TrimSpace(vars["repo"])
	}

	obj, err := repoCover(repo, tag)
	if err == nil || err == ErrCovInPrgrs || err == ErrQueued {
		loading := false
		if err == ErrCovInPrgrs || err == ErrQueued {
			loading = true
		}
		repos, err := repoLatest()
		if err != nil {
			errLogger.Println(err)
		}
		repoTmpl.Execute(w, map[string]interface{}{
			"Repo":         repo,
			"Cover":        obj.Cover,
			"Tag":          obj.Tag,
			"repositories": repos,
			"Loading":      loading,
		})
		return
	}

	if err == ErrUnknown {
		repos, _ := repoLatest()
		repoTmpl.Execute(w, map[string]interface{}{
			"Repo":         repo,
			"Cover":        ErrUnknown.Error(),
			"Tag":          obj.Tag,
			"repositories": repos,
		})
		return
	}

	errLogger.Println(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)

}

// Handler returns the homepage
func Handler(w http.ResponseWriter, r *http.Request) {
	repos, err := repoLatest()
	if err != nil {
		errLogger.Println(err)
	}
	err = homeTmpl.Execute(w, map[string]interface{}{
		"repositories": repos,
	})
	if err != nil {
		errLogger.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
