#!/bin/bash

echo "Building Go images required for testing coverage"
cp ./dockers/Golang/run.sh ./dockers/Golang/1.10/run.sh
docker build --tag bnkamalesh/cover.go:1.10 ./dockers/Golang/1.10
rm ./dockers/Golang/1.10/run.sh

cp ./dockers/Golang/run.sh ./dockers/Golang/1.9/run.sh
docker build --tag bnkamalesh/cover.go:1.9 ./dockers/Golang/1.9
rm ./dockers/Golang/1.9/run.sh

cp ./dockers/Golang/run.sh ./dockers/Golang/1.8/run.sh
docker build --tag bnkamalesh/cover.go:1.8 ./dockers/Golang/1.8
rm ./dockers/Golang/1.8/run.sh