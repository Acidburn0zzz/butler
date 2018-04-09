#!/bin/sh -xe

go version

export CURRENT_BUILD_PATH=$(pwd)
export GOPATH=$CURRENT_BUILD_PATH
export PKG=github.com/itchio/butler

mkdir -p src/$PKG
rsync -a --exclude 'src' . src/$PKG || echo "rsync complained (code $?)"
go get -v -d -t $PKG/...
go test -v $PKG/...

