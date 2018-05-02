package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestImageSupported(t *testing.T) {
	tt := []string{"golang-1.10", "golang-1.9", "golang-1.8"}
	for _, tag := range tt {
		if !imageSupported(tag) {
			t.Log(tag, " should be suported")
			t.Fail()
		}
	}

	if imageSupported("golang-1.7") {
		t.Log("golang-1.7 should not be suported")
		t.Fail()
	}
}

func TestGetBadge(t *testing.T) {
	expected := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="104" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="104" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h61v20H0z"/><path fill="#e05d44" d="M61 0h43v20H61z"/><path fill="url(#b)" d="M0 0h104v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"><text x="315" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">cover.run</text><text x="315" y="140" transform="scale(.1)" textLength="510">cover.run</text><text x="815" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="330">100%</text><text x="815" y="140" transform="scale(.1)" textLength="330">100%</text></g> </svg>`
	bytes, err := getBadge("red", "flat", "100%")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if string(bytes) != expected {
		t.Log("Expected svg badge, got", string(bytes))
		t.Fail()
	}
}

func TestRun(t *testing.T) {
	_, stderr, err := run("avelino/cover.run", "golang-1.10", "github.com/avelino/cover.run")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		t.Log(stderr)
		t.Fail()
	}
}

func TestRepoCover(t *testing.T) {
	_, err := repoCover("github.com/avelino/cover.run", "golang-1.10")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestRepoLatest(t *testing.T) {
	_, err := repoLatest()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func setup() (*mux.Router, *httptest.ResponseRecorder) {
	r := mux.NewRouter()
	r.HandleFunc("/", Handler)
	r.HandleFunc("/go/{repo:.*}.json", HandlerRepoJSON)
	r.HandleFunc("/go/{repo:.*}.svg", HandlerRepoSVG)
	r.HandleFunc("/go/{repo:.*}", HandlerRepo)
	return r, httptest.NewRecorder()
}
func TestHandler(t *testing.T) {
	router, respRec := setup()
	url := "http://localhost/"

	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(nil))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	router.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Log("Expected 200, got", respRec.Code)
		t.Fail()
	}
}

func TestHandlerRepoJSON(t *testing.T) {
	router, respRec := setup()
	url := "http://localhost/go/github.com/avelino/cover.run.json?tag=golang-1.10"

	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(nil))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	router.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Log("Expected 200, got", respRec.Code)
		t.Fail()
	}
}

func TestHandlerRepoSVG(t *testing.T) {
	router, respRec := setup()
	url := "http://localhost/go/github.com/avelino/cover.run.svg?tag=golang-1.10"

	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(nil))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	router.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Log("Expected 200, got", respRec.Code)
		t.Fail()
	}
}

func TestHandlerRepo(t *testing.T) {
	router, respRec := setup()
	url := "http://localhost/go/github.com/avelino/cover.run?tag=golang-1.10"

	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(nil))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	router.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Log("Expected 200, got", respRec.Code)
		t.Fail()
	}
}
