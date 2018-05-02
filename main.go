package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"github.com/go-redis/cache"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/nuveo/gofn"
	"github.com/nuveo/gofn/provision"
	"github.com/urfave/negroni"
)

var (
	errLogger = log.New(os.Stderr, "Cover.Run ", log.LstdFlags|log.Lshortfile)

	httpClient = &http.Client{
		// img.shields.io response time is very slow
		Timeout: 30 * time.Second,
	}

	// ErrImgUnSupported is the error returned when the Go version requested is
	// not in the supported list
	ErrImgUnSupported = errors.New("Unsupported Go version provided")
)

var (
	redisRing = redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": "redis:6379",
		},
	})

	redisCodec = &cache.Codec{
		Redis: redisRing,

		Marshal: func(v interface{}) ([]byte, error) {
			return msgpack.Marshal(v)
		},
		Unmarshal: func(b []byte, v interface{}) error {
			return msgpack.Unmarshal(b, v)
		},
	}
)

var (
	repoTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))
	homeTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))
)

const (
	// DefaultTag is the Go version to run the tests with when no version
	// is specified
	DefaultTag = "golang-1.10"
)

func imageSupported(tag string) bool {
	switch tag {
	case
		"golang-1.10",
		"golang-1.9",
		"golang-1.8":
		return true
	}
	return false
}

func run(imageRepoName, dockerTag, repo string) (string, string, error) {
	buildOpts := &provision.BuildOptions{
		DoNotUsePrefixImageName: true,
		ImageName:               strings.ToLower(fmt.Sprintf("%s:%s", imageRepoName, dockerTag)),
		StdIN:                   fmt.Sprintf("sh /run.sh %s", repo),
	}

	StdOut, StdErr, err := gofn.Run(buildOpts, &provision.ContainerOptions{})
	if err != nil {
		errLogger.Println(err, buildOpts)
	}

	return StdOut, StdErr, err
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

// Object struct holds all the details of a repository
type Object struct {
	Repo   string
	Tag    string
	Cover  string
	Output bool
}

// repoCover returns code coverage details for the given repository and Go version
func repoCover(repo, imageTag string) (*Object, error) {
	obj := &Object{}
	cacheKey := fmt.Sprintf("%s-%s", repo, imageTag)
	obj.Repo = repo
	obj.Tag = imageTag
	if !imageSupported(imageTag) {
		obj.Cover = fmt.Sprintf("Sorry, docker image not found, avelino/cover.run:%s, see Supported languages: https://github.com/avelino/cover.run#supported", imageTag)
		return obj, ErrImgUnSupported
	}

	if err := redisCodec.Get(cacheKey, &obj); err != nil {
		StdOut, StdErr, err := run("avelino/cover.run", imageTag, repo)
		stdOut := strings.Trim(StdOut, " \n")
		obj.Cover = StdErr
		obj.Output = false
		if stdOut != "" {
			obj.Cover = stdOut
			obj.Output = true
		}
		redisCodec.Set(&cache.Item{
			Key:        cacheKey,
			Object:     obj,
			Expiration: time.Hour,
		})

		return obj, err
	}
	return obj, nil
}

type Repository struct {
	Repo  string
	Tag   string
	Cover string
}

func repoLatest() ([]*Repository, error) {
	repos := make([]*Repository, 0)
	keys, _, err := redisRing.Scan(0, "*", 10).Result()
	if err != nil {
		errLogger.Println(err)
		return repos, err
	}

	var obj Object
	for _, key := range keys {
		if len(repos) == 5 {
			return repos, nil
		}
		if err := redisCodec.Get(key, &obj); err == nil {
			if obj.Output {
				repos = append(repos, &Repository{obj.Repo, obj.Tag, obj.Cover})
			}
		}
	}
	return repos, nil
}

// HandlerRepoJSON returns the coverage details of a repository as JSON
func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}
	obj, err := repoCover(vars["repo"], tag)
	if err != nil {
		errLogger.Println(err)
		json.NewEncoder(w).Encode(nil)
		return
	}
	json.NewEncoder(w).Encode(obj)
}

func HandlerRepoSVG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-CoverRunProxy", "CoverRunProxy")
	w.Header().Set("cache-control", "priviate, max-age=0, no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")

	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}

	badgeStyle := r.URL.Query().Get("style")
	if badgeStyle != "flat" {
		badgeStyle = "flat-square"
	}

	obj, err := repoCover(vars["repo"], tag)
	if err != nil {
		errLogger.Println(err)
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

	badgeName := fmt.Sprintf("%s%s%s", color, badgeStyle, obj.Cover)
	svg, err := redisRing.Get(badgeName).Bytes()
	if err != nil {
		if err != redis.Nil {
			errLogger.Print("badge cache lookup: ", err)
		}

		svg, err = getBadge(color, badgeStyle, obj.Cover)
		if err != nil {
			errLogger.Print("badge retrieve: ", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		go func() {
			err := redisRing.Set(badgeName, svg, 0).Err()
			if err != nil {
				errLogger.Print("badge store: ", err)
			}
		}()
	}

	w.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
	w.Header().Set("Content-Encoding", "br")
	w.Write(svg)
}

func HandlerRepo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["repo"]
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DefaultTag
	}

	obj, err := repoCover(repo, tag)
	if err != nil {
		errLogger.Println(err)
		return
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
	})
}

func Handler(w http.ResponseWriter, r *http.Request) {
	repos, err := repoLatest()
	if err != nil {
		errLogger.Println(err)
	}

	homeTmpl.Execute(w, map[string]interface{}{
		"repositories": repos,
	})
}

func main() {
	n := negroni.Classic()
	r := mux.NewRouter()
	r.HandleFunc("/", Handler)
	r.HandleFunc("/go/{repo:.*}.json", HandlerRepoJSON)
	r.HandleFunc("/go/{repo:.*}.svg", HandlerRepoSVG)
	r.HandleFunc("/go/{repo:.*}", HandlerRepo)
	r.PathPrefix("/assets").Handler(
		http.StripPrefix("/assets", http.FileServer(http.Dir("./assets/"))))
	n.UseHandler(r)
	n.Run(":3000")
}
