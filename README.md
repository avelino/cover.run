[![Build Status](https://travis-ci.org/avelino/cover.run.svg?branch=master)](https://travis-ci.org/avelino/cover.run)

# [cover.run](https://cover.run)

cover - Generate test coverage badge for any public Go package. Supported Go versions

- 1.11
- 1.10
- 1.9
- 1.8

### Supported badge styles

- flat ![coverage](https://cover.run/badge?color=yellow&style=flat&value=75.5%25)
- flat-square ![coverage](https://cover.run/badge?color=red&style=flat-square&value=10%25)

Style is specified as a query string parameter, e.g. `https://cover.run/github.com/avelino/cover.run.svg?style=flat-square`

### Pre-requisites

1. Docker
2. Docker compose

### How to run?

```bash
$ cd $GOPATH/src/github.com/avelino/cover.run
# build.sh builds all the required Docker images for running the tests
$ ./docker/build.sh
$ docker-compose up
```
