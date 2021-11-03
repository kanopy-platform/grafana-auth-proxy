all: main.go internal/cli/cli.go go.mod Dockerfile build/Dockerfile-test .drone.yml self-destruct

export GO_VERSION := 1.16
export GOLANGCI_IMAGE := golangci/golangci-lint:v1.38.0-alpine
export DEFAULT_APP_PORT := 8080

export GIT_ROOT := $(shell git rev-parse --show-toplevel)
export GO_MODULE := $(shell git config --get remote.origin.url | grep -o 'github\.com[:/][^.]*' | tr ':' '/')
export CMD_NAME := $(shell basename ${GO_MODULE})

.PHONY: self-destruct
self-destruct:
	./template/Makefile.sh
	./template/README.md.sh
	rm -rf template/

.PHONY: clean
clean:
	rm -rf go.mod go.sum main.go internal/cli/ Dockerfile .drone.yml build/

go.mod:
	./template/go.mod.sh
	go mod tidy

main.go:
	./template/main.go.sh

internal/cli/cli.go:
	mkdir -p internal/cli
	./template/cli.go.sh

Dockerfile:
	./template/Dockerfile.sh

build/Dockerfile-test:
	mkdir -p build
	./template/Dockerfile-test.sh

.drone.yml:
	./template/.drone.yml.sh