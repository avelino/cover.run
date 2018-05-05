package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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
	// coverQName is the Redis channel name where the requests are queued
	coverQName = "coverqueue"

	// inProgrsKey is the redis HSet key in which all repo + tags which are currently being run
	// are saved
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

	repoTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))
	homeTmpl = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))
)

// imageSupported returns true if the given Go version is supported
func imageSupported(tag string) bool {
	switch tag {
	case "golang-1.10",
		"golang-1.9",
		"golang-1.8":
		return true
	}
	return false
}

// repoExists checks if the given repository exists (works only if HTTP request returns 200)
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
	return true, nil
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

// repoFullName generates a name by combining the Go tag
func repoFullName(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

// repoTagFromFullName returns the repo name and Go tag, given the generated full name
func repoTagFromFullName(msg string) (string, string) {
	parts := strings.Split(msg, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// addToQ pushes a new cover run request to the Redis channel
func addToQ(repo, tag string) error {
	qLock.Lock()
	err := redisClient.Publish(coverQName, repoFullName(repo, tag)).Err()
	qLock.Unlock()
	return err
}

// repoCoverStatus returns true if a repository + tag cover run is in progress
func repoCoverStatus(repo, tag string) (bool, error) {
	// Check if cover run is already in progress for the given repo and tag
	err := redisRing.HGet(inProgrsKey, repoFullName(repo, tag)).Err()
	if err == nil {
		return true, nil
	}
	return false, err
}

// setInProgress sets the repo + tag as in progress by adding to inPrgorsKey
func setInProgress(repo, tag string) error {
	err := redisRing.HSet(inProgrsKey, repoFullName(repo, tag), "y").Err()
	if err != nil {
		errLogger.Println(err)
	}
	return err
}

// unsetInProgress unsets the repo + tag from inprogress status
func unsetInProgress(repo, tag string) error {
	err := redisRing.HDel(inProgrsKey, repoFullName(repo, tag)).Err()
	if err != nil {
		errLogger.Println(err)
	}
	return err
}

// cover evaluates the coverage of a repository
// - Before starting evaluation, it sets the repo's status as in progress
// - Removes the inprogress status of a repo after it's done
func cover(repo, tag string) error {
	atomic.AddInt32(&coverQCur, 1)

	err := setInProgress(repo, tag)
	if err != nil {
		errLogger.Println(err)
	}

	StdOut, StdErr, err := run("avelino/cover.run", tag, repo)
	if err != nil {
		errLogger.Println(err)
		err := unsetInProgress(repo, tag)
		if err != nil {
			errLogger.Println(err)
		}
		if coverQCur > -1 {
			atomic.AddInt32(&coverQCur, -1)
		}
		return err
	}

	err = unsetInProgress(repo, tag)
	if err != nil {
		errLogger.Println(err)
	}

	obj := &Object{
		Repo:   repo,
		Tag:    tag,
		Cover:  StdErr,
		Output: false,
	}

	stdOut := strings.Trim(StdOut, " \n")
	if stdOut != "" {
		obj.Cover = stdOut
		obj.Output = true
	}

	err = redisCodec.Set(&cache.Item{
		Key:        repoFullName(repo, tag),
		Object:     obj,
		Expiration: time.Hour,
	})
	if err != nil {
		errLogger.Println(err)
	}

	// reduce the simultaneous process number by 1
	if coverQCur > -1 {
		atomic.AddInt32(&coverQCur, -1)
	}
	return nil
}

// repoCover returns code coverage details for the given repository and Go version
// - It checks if the coverage details is available in cache or not
// - It checks if the cover run is in progress or not
// - It checks if cover can be run simultaneously, if not request is pushed to Q
func repoCover(repo, imageTag string) (*Object, error) {
	obj := &Object{
		Repo: repo,
		Tag:  imageTag,
	}

	if redisCodec.Get(repoFullName(repo, imageTag), &obj) == nil {
		return obj, nil
	}

	if !imageSupported(imageTag) {
		obj.Cover = fmt.Sprintf("Sorry, docker image not found, avelino/cover.run:%s, see Supported languages: https://github.com/avelino/cover.run#supported", imageTag)
		return obj, ErrImgUnSupported
	}

	if ok, _ := repoCoverStatus(repo, imageTag); ok {
		return obj, ErrCovInPrgrs
	}

	// Checking if limit of simultaneous processing has reached or not
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

// subscribe subscribes to the Redis channel
func subscribe(qname string) {
	pubsub, err := redisClient.Subscribe(qname)
	defer pubsub.Close()
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
		repo, tag := repoTagFromFullName(msg.Payload)
		if ok, _ := repoCoverStatus(repo, tag); !ok {
			cover(repo, tag)
		}
	}
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
	go subscribe("coverQueue")
	n.Run(":3000")
}
