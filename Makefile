.PHONY: build build-alpine clean test help default docker-build

BIN_NAME=bashhub-server
VERSION=$(shell git tag | sort --version-sort -r | head -1)
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
BUILD_DATE=$(shell date '+%Y-%m-%d-%H:%M:%S')
IMAGE_NAME := "nicksherron/bashhub-server"

default: test

help:
	@echo 'Management commands for bashhub-server:'
	@echo
	@echo 'Usage:'
	@echo '    make build           Compile the project.'
	@echo '    make get-deps        runs dep ensure, mostly used for ci.'
	@echo '    make build-alpine    Compile optimized for alpine linux.'
	@echo '    make package         Build final docker image with just the go binary inside'
	@echo '    make tag             Tag image created by package with latest, git commit and version'
	@echo '    make test            Run tests on a compiled project.'
	@echo '    make push            Push tagged images to registry'
	@echo '    make clean           Clean the directory tree.'
	@echo

build:
	@echo "building ${BIN_NAME} ${VERSION}"
	@echo "GOPATH=${GOPATH}"
	go build  -ldflags "-X github.com/nicksherron/bashhub-server/cmd.Version=${VERSION} -X github.com/nicksherron/bashhub-server/cmd.GitCommit=${GIT_COMMIT} -X github.com/nicksherron/bashhub-server/cmd.BuildDate=${BUILD_DATE}" -o bin/${BIN_NAME}

get-deps:
	dep ensure


docker-build:
	docker build --no-cache=true --build-arg VERSION=${VERSION} --build-arg BUILD_DATE=${BUILD_DATE} --build-arg GIT_COMMIT=${GIT_COMMIT}  -t $(IMAGE_NAME) .


build-alpine:
	@echo "building ${BIN_NAME} ${VERSION}"
	@echo "GOPATH=${GOPATH}"
	go build -ldflags '-w -linkmode external -extldflags "-static" -X github.com/nicksherron/bashhub-server/cmd.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X github.com/nicksherron/bashhub-server/cmd.BuildDate=${BUILD_DATE}' -o bin/${BIN_NAME}

package:
	@echo "building image ${BIN_NAME} ${VERSION} $(GIT_COMMIT)"
	docker build --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=$(GIT_COMMIT) -t $(IMAGE_NAME):local .

tag: 
	@echo "Tagging: latest ${VERSION} $(GIT_COMMIT)"
	docker tag $(IMAGE_NAME) $(IMAGE_NAME):$(GIT_COMMIT)
	docker tag $(IMAGE_NAME) $(IMAGE_NAME):${VERSION}
	docker tag $(IMAGE_NAME) $(IMAGE_NAME):latest

push: docker-build tag
	@echo "Pushing docker image to registry: latest ${VERSION} $(GIT_COMMIT)"
	docker push $(IMAGE_NAME):$(GIT_COMMIT)
	docker push $(IMAGE_NAME):${VERSION}
	docker push $(IMAGE_NAME):latest

clean:
	@test ! -e bin/${BIN_NAME} || rm bin/${BIN_NAME}

test:
	go test ./...

