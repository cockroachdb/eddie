.PHONY: build clean generate fmt install lint test

all: build

build:
	go build ./cmd/eddie

clean:
	go clean ./... 

generate: 
	go generate ./... 

fmt:
	go fmt ./... 

install:
	go install 

lint: generate
	go mod tidy
	go run golang.org/x/lint/golint -set_exit_status ./...
	go run honnef.co/go/tools/cmd/staticcheck -checks all ./...

test: generate
	go test -vet all ./...

release: fmt lint test build
