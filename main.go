package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
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

type Object struct {
	Cover string
}

func main2() {
	repo := flag.String("repo", "", "a string")
	flag.Parse()
	run("dockers/Golang", "Dockerfile", *repo)
}

func repoCover(repo string) (obj Object) {
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": "redis:6379",
		},
	})
	codec := &cache.Codec{
		Redis: ring,

		Marshal: func(v interface{}) ([]byte, error) {
			return msgpack.Marshal(v)
		},
		Unmarshal: func(b []byte, v interface{}) error {
			return msgpack.Unmarshal(b, v)
		},
	}
	if err := codec.Get(repo, &obj); err != nil {
		StdOut, StdErr := run("dockers/Golang", "Dockerfile", repo)
		stdOut := strings.Trim(StdOut, " \n")
		if stdOut != "" {
			obj.Cover = stdOut
		} else {
			obj.Cover = StdErr
		}
		obj := &Object{
			Cover: obj.Cover,
		}
		codec.Set(&cache.Item{
			Key:        repo,
			Object:     obj,
			Expiration: time.Hour,
		})
	}
	return
}

func HandlerRepoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	obj := repoCover(vars["repo"])
	json.NewEncoder(w).Encode(obj)
}

func HandlerRepo(w http.ResponseWriter, r *http.Request) {
	Body := map[string]interface{}{}
	vars := mux.Vars(r)
	repo := vars["repo"]
	Body["Repo"] = repo
	obj := repoCover(repo)
	Body["Cover"] = obj.Cover
	t := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/repo.tmpl"))
	t.Execute(w, Body)
	return

}

func Handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/home.tmpl"))
	t.Execute(w, nil)
	return
}

func main() {
	n := negroni.Classic()
	r := mux.NewRouter()
	r.HandleFunc("/", Handler)
	r.HandleFunc("/go/{repo:.*}.json", HandlerRepoJSON)
	r.HandleFunc("/go/{repo:.*}", HandlerRepo)
	r.PathPrefix("/assets").Handler(
		http.StripPrefix("/assets", http.FileServer(http.Dir("./assets/"))))
	n.UseHandler(r)
	n.Run(":3000")
}

func run(contextDir, dockerFile, repo string) (StdOut, StdErr string) {
	buildOpts := &provision.BuildOptions{
		ContextDir: contextDir,
		Dockerfile: dockerFile,
		ImageName:  strings.ToLower(fmt.Sprintf("%s/%s", contextDir, dockerFile)),
		StdIN:      fmt.Sprintf("sh /run.sh %s", repo),
	}
	StdOut, StdErr, err := gofn.Run(
		buildOpts,
		&provision.VolumeOptions{})
	if err != nil {
		log.Println(err)
	}
	return
}
