FROM golang:1.10

RUN mkdir -p /go/src/github.com/bnkamalesh/cover.run
ADD  ./ /go/src/github.com/bnkamalesh/cover.run
WORKDIR /go/src/github.com/bnkamalesh/cover.run
RUN go build
CMD ["./cover.run"]
