#!/bin/bash
set -e

trap 'echo "[ERROR] exitcode=$?"' ERR

info() {
  echo "[INFO] $1"
}

DOCKER_IMAGE="daolabs/undertaker:latest"
info "executing go get ..."
./tools/go-builder get

info "building statically linked binary [undertaker] ..."
CGO_ENABLED=0 ./tools/go-builder build -a --ldflags='-s'

info "building [$DOCKER_IMAGE] ..."
docker build -t "$DOCKER_IMAGE" .
GREP_TARGET=${DOCKER_IMAGE/\//\\/}
GREP_TARGET=${GREP_TARGET/:latest/}
docker images | grep "REPOSITORY\|$GREP_TARGET"

info "done."