#!/usr/bin/env bash

# script to build x86 docker images for local usage.
# we're assuming that you are using a an x86 machine.

BASE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT=$(realpath "${BASE}/../")

docker build -t edgerun/go-load-balancer -f build/package/go-load-balancer/Dockerfile.amd64 .