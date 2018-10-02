all: build

.PHONY: clean
clean:
	rm -rf isucoin

init:
	GOPATH=`pwd`; DEP_RELEASE_TAG=v0.5.0 curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

deps:
	GOPATH=`pwd`; cd ${GOPATH}/src/isucon8/isucoin; ${GOPATH}/bin/dep ensure

.PHONY: build
build:
	GOPATH=`pwd` go build -v -o isucoin isucon8/isucoin/webapp 
