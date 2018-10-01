all: build

.PHONY: clean
clean:
	rm -rf isucoin

init:
	GOPATH=`pwd`; curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

deps:
	GOPATH=`pwd`; cd ${GOPATH}/src/isucon8/isucoin; dep ensure

.PHONY: build
build:
	GOPATH=`pwd` go build -v -o isucoin isucon8/isucoin/webapp 
