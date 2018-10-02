DIR = $(shell pwd)
all: build

.PHONY: clean
clean:
	rm -rf isucoin

init:
	mkdir -p ${DIR}/bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | GOPATH=${DIR} DEP_RELEASE_TAG=v0.5.0 sh

deps:
	cd ${DIR}/src/isucon8/isucoin; GOPATH=${DIR} ${DIR}/bin/dep ensure

.PHONY: build
build:
	GOPATH=${DIR} go build -v -o isucoin isucon8/isucoin/webapp
