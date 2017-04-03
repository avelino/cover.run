FROM golang:1.8

RUN go get golang.org/x/tools/cmd/cover

ADD run.sh /
RUN chmod +x /run.sh

WORKDIR /go/src

# ENTRYPOINT ["/run.sh"]
CMD ["bash"]
