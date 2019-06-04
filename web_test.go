// +build integration

package main

import (
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	_, stderr, err := run("golang-1.10", "github.com/avelino/cover.run")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		t.Log(stderr)
		t.Fail()
	}

	_, _, err = run("golang-1.0", "github.com/avelino/cover.run")
	if err.Error() != "missing remote repository e.g. 'github.com/user/repo'" {
		t.Log(err)
		t.Fail()
	}

	_, _, err = run("golang-1.10", "github.com/avelino/nonexistent")
	if err != ErrRepoNotFound {
		t.Log("Expected", ErrRepoNotFound, "got", err)
		t.Fail()
	}
}

func TestRepoCover(t *testing.T) {
	_, err := repoCover("github.com/avelino/cover.run", "golang-1.10")
	if err != nil && err != ErrCovInPrgrs && err != ErrQueued {
		t.Log(err)
		t.Fail()
	}

	_, err = repoCover("github.com/avelino/cover.run", "golang-1.0.1")
	if err != ErrImgUnSupported {
		t.Log("Expected error ", ErrImgUnSupported, "got", err)
		t.Fail()
	}

	_, err = repoCover("github.com/avelino/nonexistent", "golang-1.10")
	if err != ErrRepoNotFound && err != ErrCovInPrgrs && err != ErrQueued {
		t.Log("Expected", ErrRepoNotFound, "got", err)
		t.Fail()
	}
}
func TestCover(t *testing.T) {
	qChan <- struct{}{}
	err := cover("github.com/avelino/cover.run", "golang-1.10")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	qChan <- struct{}{}
	err = cover("github.com/avelino/cover.run", "golang-1.0.1")
	if err == nil {
		t.Log("Expected error ", "got", err)
		t.Fail()
	}

	qChan <- struct{}{}
	err = cover("github.com/avelino/nonexistent", "golang-1.10")
	if err != ErrRepoNotFound {
		t.Log("Expected", ErrRepoNotFound, "got", err)
		t.Fail()
	}
}
