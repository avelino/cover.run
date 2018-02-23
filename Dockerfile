FROM golang:1.10

RUN mkdir -p /go/src/github.com/avelino/cover.run
COPY  ./ /go/src/github.com/avelino/cover.run
WORKDIR /go/src/github.com/avelino/cover.run
RUN go get -u github.com/kardianos/govendor
RUN govendor sync
RUN go build
CMD ["./cover.run"]
