[![Build Status](https://travis-ci.org/bnkamalesh/cover.run.svg?branch=master)](https://travis-ci.org/bnkamalesh/cover.run)

# [gocover.run](https://gocover.run)

gocover - Generate test coverage badge for any public Go package. Supported Go versions

- 1.10
- 1.9
- 1.8

### Supported badge styles

- flat [![coverage](https://gocover.run/github.com/bnkamalesh/cover.run.svg?style=flat)]
- flat-square [![coverage](https://gocover.run/github.com/bnkamalesh/cover.run.svg?style=flat-square)]

Style is specified as a query string parameter, e.g. `https://gocover.run/github.com/bnkamalesh/cover.run.svg?style=flat-square`

### Pre-requisites

1. Docker
2. Docker compose

### How to run?

```bash
$ cd $GOPATH/src/github.com/bnkamalesh/cover.run
$ docker-compose up
```