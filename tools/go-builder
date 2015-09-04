#!/bin/sh

IMAGE_TO_USE="sys42/docker-go-builder:latest"

TMPENV=""
[ -n "$GOOS" ]        && TMPENV="-e GOOS=$GOOS"
[ -n "$GOARCH" ]      && TMPENV="$TMPENV -e GOARCH=$GOARCH"
[ -n "$CGO_ENABLED" ] && TMPENV="$TMPENV -e CGO_ENABLED=$CGO_ENABLED"
[ -n "$TRACE" ]       && TMPENV="$TMPENV -e TRACE=$TRACE"

[ -n "$TRACE" ] && echo "TMPENV = [$TMPENV]"


docker run -ti --rm -v $PWD:/app -w /app $TMPENV "$IMAGE_TO_USE" \
       remapuser app $(id -u) $(id -g) goexec "$@"
