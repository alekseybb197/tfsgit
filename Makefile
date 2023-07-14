PROJECT=$(shell basename "$(PWD)")
APPVERS=0.1.2
GOFLAGS=-ldflags="-w -s" -trimpath -ldflags "-X main.version=${APPVERS}"
GO111MODULE=on

default: build

.PHONY: build
build:
	go build ${GOFLAGS} -o dist/${PROJECT}

.PHONY: run
run:
	go run ${PROJECT}.go

.PHONY: dist
dist:
	# FreeBDS
	GOOS=freebsd GOARCH=amd64 go build ${GOFLAGS} -o dist/${PROJECT}-freebsd-amd64
	# MacOS
	GOOS=darwin GOARCH=amd64 go build ${GOFLAGS} -o dist/${PROJECT}-darwin-amd64
	# Linux
	GOOS=linux GOARCH=amd64 go build ${GOFLAGS} -o dist/${PROJECT}-linux-amd64
	# Windows
	GOOS=windows GOARCH=amd64 go build ${GOFLAGS} -o dist/${PROJECT}-windows-amd64.exe

.PHONY: clean
clean:
	go clean
	rm -rf dist

PHONY: fmt
fmt:
	gofumpt -w -s  .

PHONY: lint
lint:
	golangci-lint run -c .golang-ci.yml
