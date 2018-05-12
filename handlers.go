package main

import (
	"encoding/json"
	"html/template"
	"net/http"

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
	repo := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}

	vars := mux.Vars(r)
	if repo == "" {
		repo = vars["repo"]
	}

	obj, err := repoCover(repo, tag)
	if err == nil || err == ErrCovInPrgrs || err == ErrQueued {
		repos, err := repoLatest()
		if err != nil {
			errLogger.Println(err)
		}
		repoTmpl := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))
		repoTmpl.Execute(w, map[string]interface{}{
			"Repo":         repo,
			"Cover":        obj.Cover,
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
	homeTmpl := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))
	err = homeTmpl.Execute(w, map[string]interface{}{
		"repositories": repos,
	})
	if err != nil {
		errLogger.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
