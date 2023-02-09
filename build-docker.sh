#!/bin/bash

#docker run -ti --rm \
#  -v ~/tmp:/tmp \
#  -v ~/tmp/rpmbuild:/root/rpmbuild \
#  -v $PWD/:/app     \
#  golang:1.20.0-bullseye \
#  /bin/bash


##
# cd /app

declare -a _BUILDS=(
  linux@386,amd64,arm,arm64
  darwin@amd64,arm64 # cputimings not implmented # CGO_ENABLED=1 build on mac ?
)

for i in "${_BUILDS[@]}"; do
  IFS=@ read GOOS ARCHS <<< $i

  IFS=,
  for ARCH in ${ARCHS}; do
      echo "Building: ${GOOS}/${ARCH}"
      GOOS=${GOOS} GOARCH=${ARCH} \
      go build -ldflags "-w -s" \
        -o build/${GOOS}/${ARCH}/check-cpu-usage
  done

done
