#!/bin/bash
docker run -ti --rm  -v /var/run/docker.sock:/var/run/docker.sock daolabs/undertaker:latest "$@"
