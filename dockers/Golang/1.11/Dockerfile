FROM golang:1.11

COPY ./run.sh /
RUN go get golang.org/x/tools/cmd/cover \
    && chmod +x /run.sh

WORKDIR /go/src

CMD ["bash"]
