package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
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

const (
	DEFAULT_TAG = "golang-1.10"
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

type Object struct {
	Repo   string
	Tag    string
	Cover  string
	Output bool
}

var redisRing = redis.NewRing(&redis.RingOptions{
	Addrs: map[string]string{
		"server1": "redis:6379",
	},
})

var redisCodec = &cache.Codec{
	Redis: redisRing,

	Marshal: func(v interface{}) ([]byte, error) {
		return msgpack.Marshal(v)
	},
	Unmarshal: func(b []byte, v interface{}) error {
		return msgpack.Unmarshal(b, v)
	},
}

func repoCover(repo, imageTag string) (obj Object) {
	cacheKey := fmt.Sprintf("%s-%s", repo, imageTag)
	obj.Repo = repo
	obj.Tag = imageTag
	if !imageSupported(imageTag) {
		obj.Cover = fmt.Sprintf("Sorry, not found docker image avelino/cover.run:%s, see Supported languages: https://github.com/avelino/cover.run#supported", imageTag)
		return
	}
	if err := redisCodec.Get(cacheKey, &obj); err != nil {
		StdOut, StdErr := run("avelino/cover.run", imageTag, repo)
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
	}
	return
}

func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DEFAULT_TAG
	}
	obj := repoCover(vars["repo"], tag)
	json.NewEncoder(w).Encode(obj)
}

type copier struct {
	transport http.RoundTripper
}

func (c copier) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.transport.RoundTrip(request)
	return response, err
}

type Repository struct {
	Repo  string
	Tag   string
	Cover string
}

func repoLatest() (repos []*Repository) {
	keys, _, err := redisRing.Scan(0, "*", 10).Result()
	if err != nil {
		log.Println(err)
	}

	var obj Object
	for _, key := range keys {
		if len(repos) == 5 {
			return
		}
		if err := redisCodec.Get(key, &obj); err == nil {
			if obj.Output {
				repos = append(repos, &Repository{obj.Repo, obj.Tag, obj.Cover})
			}
		}
	}
	return
}

func HandlerRepoSVG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-CoverRunProxy", "CoverRunProxy")
	w.Header().Set("cache-control", "priviate, max-age=0, no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")

	vars := mux.Vars(r)
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DEFAULT_TAG
	}

	badgeStyle := r.URL.Query().Get("style")
	if badgeStyle != "flat" {
		badgeStyle = "flat-square"
	}

	obj := repoCover(vars["repo"], tag)
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
	badgeSVG, err := getBadgeCache(badgeName)
	if err == nil && len(badgeSVG) > 0 {
		serveBadge(w, badgeSVG)
		return
	}

	badge, err := getBadge(color, badgeStyle, obj.Cover)
	badgeSVG = string(badge)
	if err == nil {
		go setBadgeCache(badgeName, badgeSVG)
		setBadgeCache(badgeName, badgeSVG)
	}
	serveBadge(w, badgeSVG)
}

var repoTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))

func HandlerRepo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["repo"]
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = DEFAULT_TAG
	}
	obj := repoCover(repo, tag)

	repoTmpl.Execute(w, map[string]interface{}{
		"Repo":         repo,
		"Cover":        obj.Cover,
		"Tag":          obj.Tag,
		"repositories": repoLatest(),
	})
}

var homeTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))

func Handler(w http.ResponseWriter, r *http.Request) {
	homeTmpl.Execute(w, map[string]interface{}{
		"repositories": repoLatest(),
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

func run(imageRepoName, dockerTag, repo string) (StdOut, StdErr string) {
	buildOpts := &provision.BuildOptions{
		DoNotUsePrefixImageName: true,
		ImageName:               strings.ToLower(fmt.Sprintf("%s:%s", imageRepoName, dockerTag)),
		StdIN:                   fmt.Sprintf("sh /run.sh %s", repo),
	}
	containerOpts := &provision.ContainerOptions{}
	StdOut, StdErr, err := gofn.Run(
		buildOpts, containerOpts)
	if err != nil {
		log.Println(err)
	}

	return
}
