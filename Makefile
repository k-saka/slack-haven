.PHONY: help clean fmt build run

help:
	@echo 'clean -- remove binary'
	@echo 'fmt -- gofmt'
	@echo 'vet -- go vet'
	@echo 'build -- go build'
	@echo 'run -- execute'

clean:
	go clean

fmt:
	gofmt -w -s $(wildcard *.go)

vet:
	go vet -x $(wildcard *.go)

lint:
	golint $(wildcard *.go)

build:clean
	go build -x -v github.com/k-saka/slack-haven

run:build
	./slack-haven

install:clean
	go install github.com/k-saka/slack-haven
