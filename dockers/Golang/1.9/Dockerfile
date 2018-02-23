FROM golang:1.9

RUN go get golang.org/x/tools/cmd/cover && \
    wget https://raw.githubusercontent.com/avelino/cover.run/master/dockers/Golang/run.sh -O /run.sh && \
    chmod +x /run.sh

WORKDIR /go/src

CMD ["bash"]
