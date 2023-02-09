#!/bin/bash

docker run -ti --rm \
  -v ~/tmp:/tmp \
  -v ~/tmp/rpmbuild:/root/rpmbuild \
  -v $PWD/:/app     \
  golang:1.20.0-bullseye \
  /bin/bash


##
cd /app
go build -o check-cpu-usage