package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/nuveo/gofn"
	"github.com/nuveo/gofn/provision"
	"github.com/urfave/negroni"
)

func main2() {
	repo := flag.String("repo", "", "a string")
	flag.Parse()
	run("dockers/Golang", "Dockerfile", *repo)
}

func HandlerRepo(w http.ResponseWriter, r *http.Request) {
	Body := map[string]interface{}{}
	vars := mux.Vars(r)
	Body["Repo"] = vars["repo"]
	StdOut, StdErr := run("dockers/Golang", "Dockerfile", vars["repo"])
	stdOut := strings.Trim(StdOut, " \n")
	if stdOut != "" {
		Body["Cover"] = stdOut
	} else {
		Body["Cover"] = StdErr
	}

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
