.PHONY: help clean fmt build run test check

REVISION := $(shell git rev-parse --verify HEAD)

help:
	@echo 'clean -- remove binary'
	@echo 'fmt -- gofmt'
	@echo 'vet -- go vet'
	@echo 'build -- go build'
	@echo 'check -- format and static check'
	@echo 'run -- execute'
	@echo 'test -- go test'

clean:
	go clean

fmt:
	gofmt -s -w -l ./

vet:
	go tool vet ./

lint:
	golint ./

check:fmt vet lint

build:clean
	go build -x -v -ldflags "-X main.version=$(REVISION)" github.com/k-saka/slack-haven

run:build
	./slack-haven

install:clean
	go install github.com/k-saka/slack-haven

test:
	go test
	@cd haven; go test
