FROM golang:1.10

RUN mkdir -p /go/src/github.com/avelino/cover.run
ADD  ./ /go/src/github.com/avelino/cover.run
WORKDIR /go/src/github.com/avelino/cover.run
RUN go build
CMD ["./cover.run"]
