#!/bin/bash

echo "Building Go images required for testing coverage"
cp ./dockers/golang/run.sh ./dockers/golang/1.11/run.sh
docker build --tag avelino/cover.run:golang-1.11 ./dockers/golang/1.11
rm ./dockers/golang/1.11/run.sh

cp ./dockers/golang/run.sh ./dockers/golang/1.10/run.sh
docker build --tag avelino/cover.run:golang-1.10 ./dockers/golang/1.10
rm ./dockers/golang/1.10/run.sh

cp ./dockers/golang/run.sh ./dockers/golang/1.9/run.sh
docker build --tag avelino/cover.run:golang-1.9 ./dockers/golang/1.9
rm ./dockers/golang/1.9/run.sh

cp ./dockers/golang/run.sh ./dockers/golang/1.8/run.sh
docker build --tag avelino/cover.run:golang-1.8 ./dockers/golang/1.8
rm ./dockers/golang/1.8/run.sh
