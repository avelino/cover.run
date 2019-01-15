FROM golang:1.11

COPY . /go/src/github.com/avelino/cover.run
WORKDIR /go/src/github.com/avelino/cover.run

RUN go build
CMD ["./cover.run"]
