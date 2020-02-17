.PHONY: build build-alpine clean test help default docker-build

BIN_NAME=bashhub-server
VERSION=$(shell git tag | sort --version-sort -r | head -1)
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
BUILD_DATE=$(shell date '+%Y-%m-%d-%H:%M:%S')
IMAGE_NAME="nicksherron/bashhub-server"


default: help

help:
	@echo 'Management commands for bashhub-server:'
	@echo
	@echo 'Usage:'
	@echo '    make build           Compile the project'
	@echo '    make docker-build    Build docker image'
	@echo '    make clean           Clean the directory tree'
	@echo '    make test            Run tests on a compiled project'
	@echo '    make test-postgres   Start postgres in ephemeral docker container and run backend tests'
	@echo '    make test-all        Run test and test-postgres'
	@echo

build:
	@echo "building $(BIN_NAME) $(VERSION)"
	@echo "GOPATH=$(GOPATH)"
	go build  -ldflags "-X github.com/nicksherron/bashhub-server/cmd.Version=$(VERSION) -X github.com/nicksherron/bashhub-server/cmd.GitCommit=$(GIT_COMMIT) -X github.com/nicksherron/bashhub-server/cmd.BuildDate=$(BUILD_DATE)" -o bin/${BIN_NAME}

docker-build:
	docker build --no-cache=true --build-arg VERSION=${VERSION} --build-arg BUILD_DATE=${BUILD_DATE} --build-arg GIT_COMMIT=${GIT_COMMIT}  -t $(IMAGE_NAME) .

clean:
	@test ! -e bin/$(BIN_NAME) || rm bin/$(BIN_NAME)

test:
	go test ./...

test-postgres:
	scripts/test_postgres.sh

test-all: test test-postgres



