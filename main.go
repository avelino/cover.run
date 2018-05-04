package main

import (
	"context"
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/cache"
	"github.com/go-redis/redis"
	"github.com/gofn/gofn"
	"github.com/gofn/gofn/provision"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

const (
	// coverQMax is the maximum number of coverage run to be executed simultaneously
	coverQMax = 5

	// DefaultTag is the Go version to run the tests with when no version
	// is specified
	DefaultTag = "golang-1.10"

	// errorBadgeCurve is the SVG badge returned when coverage badge could not be returned
	errorBadgeCurve = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="100" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#e05d44" d="M61 0h39v20H61z"/><path fill="url(#b)" d="M0 0h100v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="795" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="290">failed</text><text x="795" y="140" transform="scale(.1)" textLength="290">failed</text></g> </svg>`
	errorBadgeFlat  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#e05d44" d="M61 0h39v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="795" y="140" transform="scale(.1)" textLength="290">failed</text></g> </svg>`

	// progressBadgeCurve is the SVG badge returned when coverage run is in progress
	queuedBadgeCurve = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="114" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="114" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#9f9f9f" d="M61 0h53v20H61z"/><path fill="url(#b)" d="M0 0h114v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="865" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="430">inqueue</text><text x="865" y="140" transform="scale(.1)" textLength="430">inqueue</text></g> </svg>`
	queuedBadgeFlat  = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="114" height="20"><g shape-rendering="crispEdges"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#9f9f9f" d="M61 0h53v20H61z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="865" y="140" transform="scale(.1)" textLength="430">inqueue</text></g> </svg>`
)

var (
	// errLogger is the log instance with all the required flags set for error logging
	errLogger = log.New(os.Stderr, "Cover.Run ", log.LstdFlags|log.Lshortfile)

	// coverQCur is the current number of cover run requests being executed
	coverQCur = int32(0)

	httpClient = &http.Client{
		// img.shields.io response time is very slow
		Timeout: 30 * time.Second,
	}
	// ErrImgUnSupported is the error returned when the Go version requested is
	// not in the supported list
	ErrImgUnSupported = errors.New("Unsupported Go version provided")
	// ErrRepoNotFound is the error returned when the repository does not exist
	ErrRepoNotFound = errors.New("Repository not found")
	// ErrUnknown is the error returned when an unidentified error is encountered
	ErrUnknown = errors.New("Unknown error occurred")
	// ErrQueueFull is the error returned when the cover run queueu is full
	ErrQueueFull = errors.New("Cover run queue is full")

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
	redisClient = redis.NewClient(&redis.Options{
		Addr:         "redis:6379",
		ReadTimeout:  time.Second * 2,
		DialTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
		PoolTimeout:  time.Second * 120,
	})
	qLock = sync.Mutex{}
	qChan = make(chan struct{}, coverQMax)

	repoTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))
	homeTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))
)

// imageSupported returns true if the given Go version is supported
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

func repoExists(repo string) (bool, error) {
	resp, err := httpClient.Get(fmt.Sprintf("https://%s", repo))
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, ErrRepoNotFound
	}

	if resp.StatusCode > 399 {
		return false, ErrUnknown
	}
	return false, nil
}

func run(imageRepoName, dockerTag, repo string) (string, string, error) {
	_, err := repoExists(repo)
	if err != nil {
		return "", "", err
	}

	buildOpts := &provision.BuildOptions{
		DoNotUsePrefixImageName: true,
		ImageName:               strings.ToLower(fmt.Sprintf("%s:%s", imageRepoName, dockerTag)),
		StdIN:                   fmt.Sprintf("sh /run.sh %s", repo),
	}

	// 5 minutes timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()

	StdOut, StdErr, err := gofn.Run(ctx, buildOpts, &provision.ContainerOptions{})
	if err != nil {
		errLogger.Println(err)
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

func getQMsg(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

func getRepoTagFromMsg(msg string) (string, string) {
	parts := strings.Split(msg, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func addToQ(repo, tag string) error {
	qLock.Lock()
	err := redisClient.Publish("coverQueue", getQMsg(repo, tag)).Err()
	qLock.Unlock()
	return err
}

func processQ(qname string) {
	pubsub, err := redisClient.Subscribe(qname)
	defer pubsub.Close()
	if err != nil {
		errLogger.Println(err)
		return
	}

	err = redisClient.Publish("mychannel1", "hello").Err()
	if err != nil {
		errLogger.Println(err)
		return
	}

	msg, err := pubsub.ReceiveMessage()
	if err != nil {
		errLogger.Println(err)
		return
	}

	// Check if cover run is already in progress for the given repo and tag
	if redisRing.HGet("cover-in-progress", msg.String()).Err() == nil {
		return
	}

	// Set the repo + tag as in progress to prevent the same repo+tag clogging the Q
	if redisRing.HSet("cover-in-progress", msg.String(), true).Err() != nil {
		errLogger.Println(err)
	}

	repo, tag := getRepoTagFromMsg(msg.String())
	StdOut, StdErr, err := run("avelino/cover.run", tag, repo)
	if err != nil {
		errLogger.Println(err)
	}

	if redisRing.HDel("cover-in-progress", msg.String()).Err() != nil {
		errLogger.Println(err)
	}

	obj := &Object{}
	stdOut := strings.Trim(StdOut, " \n")
	obj.Cover = StdErr
	obj.Output = false
	if stdOut != "" {
		obj.Cover = stdOut
		obj.Output = true
	}
	cacheKey := fmt.Sprintf("%s-%s", repo, tag)
	err = redisCodec.Set(&cache.Item{
		Key:        cacheKey,
		Object:     obj,
		Expiration: time.Hour,
	})

	if err != nil {
		errLogger.Println(err)
	}
	if coverQCur > -1 {
		atomic.AddInt32(&coverQCur, -1)
	}
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

	if redisCodec.Get(cacheKey, &obj) == nil {
		return obj, nil
	}

	if coverQCur >= coverQMax {
		err := addToQ(repo, imageTag)
		if err != nil {
			errLogger.Println(err)
		}

		return nil, ErrQueueFull
	}

	atomic.AddInt32(&coverQCur, 1)
	go func() {
		StdOut, StdErr, err := run("avelino/cover.run", imageTag, repo)
		if err != nil {
			errLogger.Println(err)
		}

		obj := &Object{}
		stdOut := strings.Trim(StdOut, " \n")
		obj.Cover = StdErr
		obj.Output = false
		if stdOut != "" {
			obj.Cover = stdOut
			obj.Output = true
		}
		err = redisCodec.Set(&cache.Item{
			Key:        cacheKey,
			Object:     obj,
			Expiration: time.Hour,
		})
		if err != nil {
			errLogger.Println(err)
		}
		if coverQCur > -1 {
			atomic.AddInt32(&coverQCur, -1)
		}
	}()

	return nil, nil
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

	badgeName := fmt.Sprintf("%s%s%s", color, style, obj.Cover)
	svg, err := redisRing.Get(badgeName).Bytes()
	if err != nil {
		if err != redis.Nil {
			errLogger.Println(err)
		}

		svg, err = getBadge(color, style, obj.Cover)
		if err != nil {
			errLogger.Println(err)
			if style == "flat-square" {
				return errorBadgeFlat, err
			}
			return errorBadgeCurve, err
		}

		go func() {
			err := redisRing.Set(badgeName, svg, 0).Err()
			if err != nil {
				errLogger.Println(err)
			}
		}()
	}

	if style == "flat-square" {
		return queuedBadgeFlat, nil
	}
	return queuedBadgeCurve, nil
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
