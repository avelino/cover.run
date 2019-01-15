#!/bin/bash

echo "Building Go images required for testing coverage"
cp ./build/docker/golang/run.sh ./build/docker/golang/1.11/run.sh
docker build --tag avelino/cover.run:golang-1.11 ./build/docker/golang/1.11
rm ./build/docker/golang/1.11/run.sh

cp ./build/docker/golang/run.sh ./build/docker/golang/1.10/run.sh
docker build --tag avelino/cover.run:golang-1.10 ./build/docker/golang/1.10
rm ./build/docker/golang/1.10/run.sh

cp ./build/docker/golang/run.sh ./build/docker/golang/1.9/run.sh
docker build --tag avelino/cover.run:golang-1.9 ./build/docker/golang/1.9
rm ./build/docker/golang/1.9/run.sh

cp ./build/docker/golang/run.sh ./build/docker/golang/1.8/run.sh
docker build --tag avelino/cover.run:golang-1.8 ./build/docker/golang/1.8
rm ./build/docker/golang/1.8/run.sh
