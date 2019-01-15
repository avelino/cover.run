#!/bin/bash

echo "Building Go images required for testing coverage"
cp ./docker/golang/run.sh ./docker/golang/1.11/run.sh
docker build --tag avelino/cover.run:golang-1.11 ./docker/golang/1.11
rm ./docker/golang/1.11/run.sh

cp ./docker/golang/run.sh ./docker/golang/1.10/run.sh
docker build --tag avelino/cover.run:golang-1.10 ./docker/golang/1.10
rm ./docker/golang/1.10/run.sh

cp ./docker/golang/run.sh ./docker/golang/1.9/run.sh
docker build --tag avelino/cover.run:golang-1.9 ./docker/golang/1.9
rm ./docker/golang/1.9/run.sh

cp ./docker/golang/run.sh ./docker/golang/1.8/run.sh
docker build --tag avelino/cover.run:golang-1.8 ./docker/golang/1.8
rm ./docker/golang/1.8/run.sh
