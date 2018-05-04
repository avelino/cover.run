package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
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
	coverQMax = 0

	// inProgrsKey is the redis HSet key in which all repo + tags are saved
	inProgrsKey = "cover-in-progress"

	// DefaultTag is the Go version to run the tests with when no version
	// is specified
	DefaultTag = "golang-1.10"
)

var (
	// errLogger is the log instance with all the required flags set for error logging
	errLogger = log.New(os.Stderr, "Cover.Run ", log.LstdFlags|log.Lshortfile)

	// coverQCur is the current number of cover run requests being executed simultaneously
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
	// ErrCovInPrgrs is the error returned when the repo coverage test is in progress
	ErrCovInPrgrs = errors.New("Cover run is in progress")

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

// repoExists checks if the given repository exists (worksing only if HTTP request returns 200)
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

// run runs the custom script to get the coverage details; using gofn
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

// Object struct holds all the details of a repository
type Object struct {
	Repo   string
	Tag    string
	Cover  string
	Output bool
}

// getQMsg gets the message to be send to the Redis channel
func getQMsg(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

// getRepoTagFromMsg returns the repo name and Go tag from the message received from
// Redis channel
func getRepoTagFromMsg(msg string) (string, string) {
	parts := strings.Split(msg, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// addToQ pushes a new cover run request to the Redis channel
func addToQ(repo, tag string) error {
	qLock.Lock()
	err := redisClient.Publish("coverQueue", getQMsg(repo, tag)).Err()
	qLock.Unlock()
	return err
}

// checkInProgress returns true if a repository + tag cover run is in progress
func checkInProgress(repo, tag string) (bool, error) {
	// Check if cover run is already in progress for the given repo and tag
	err := redisRing.HGet(inProgrsKey, getQMsg(repo, tag)).Err()
	if err == nil {
		return true, nil
	}
	return false, err
}

// setInProgress sets the repo + tag as in progress
func setInProgress(repo, tag string) error {
	// Set the repo + tag as in progress to prevent the same repo+tag clogging the Q
	err := redisRing.HSet(inProgrsKey, getQMsg(repo, tag), "y").Err()
	if err != nil {
		errLogger.Println(err)
	}
	return err
}

// qSubscribe subscribes to the Redis channel
func qSubscribe(qname string) {
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

	for {
		msg, err := pubsub.ReceiveMessage()
		if err != nil {
			errLogger.Println(err)
			return
		}
		repo, tag := getRepoTagFromMsg(msg.Payload)
		if ok, _ := checkInProgress(repo, tag); !ok {
			cover(repo, tag)
		}
	}
}

func cover(repo, tag string) error {
	atomic.AddInt32(&coverQCur, 1)

	err := setInProgress(repo, tag)
	if err != nil {
		errLogger.Println(err)
	}

	StdOut, StdErr, err := run("avelino/cover.run", tag, repo)
	if err != nil {
		errLogger.Println(err)
		return err
	}

	cacheKey := getQMsg(repo, tag)

	if err := redisRing.HDel(inProgrsKey, cacheKey).Err(); err != nil {
		errLogger.Println(err)
	}

	obj := &Object{
		Cover:  StdErr,
		Output: false,
	}

	stdOut := strings.Trim(StdOut, " \n")
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
	return nil
}

// repoCover returns code coverage details for the given repository and Go version
func repoCover(repo, imageTag string) (*Object, error) {
	obj := &Object{
		Repo: repo,
		Tag:  imageTag,
	}

	if redisCodec.Get(getQMsg(repo, imageTag), &obj) == nil {
		return obj, nil
	}

	if !imageSupported(imageTag) {
		obj.Cover = fmt.Sprintf("Sorry, docker image not found, avelino/cover.run:%s, see Supported languages: https://github.com/avelino/cover.run#supported", imageTag)
		return obj, ErrImgUnSupported
	}

	if ok, _ := checkInProgress(repo, imageTag); ok {
		return obj, ErrCovInPrgrs
	}

	if coverQCur >= coverQMax {
		err := addToQ(repo, imageTag)
		if err != nil {
			errLogger.Println(err)
		}
		return obj, ErrQueueFull
	}

	go cover(repo, imageTag)

	return obj, ErrCovInPrgrs
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
	go qSubscribe("coverQueue")
	n.Run(":3000")
}
