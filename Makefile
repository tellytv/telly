GO    := go
GOPATH ?= $(HOME)/go
PROMU := $(GOPATH)/bin/promu

GO                      ?= go
GOFMT                   ?= $(GO)fmt
FIRST_GOPATH            := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU                   := $(FIRST_GOPATH)/bin/promu

GOMETALINTER_BINARY     := $(FIRST_GOPATH)/bin/gometalinter
DEP_BINARY              := $(FIRST_GOPATH)/bin/dep

PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)
DOCKER_IMAGE_NAME       ?= telly
DOCKER_IMAGE_TAG        ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
DOCKER_REPO             ?= tellytv


all: dep style build test

style:
	@echo ">> checking code style"
	@$(GO) get -u github.com/alecthomas/gometalinter
	@$(GOMETALINTER_BINARY) --config=.gometalinter.json --install ./...

dep: $(DEP_BINARY)
	@echo ">> installing dependencies"
	@$(DEP_BINARY) ensure -vendor-only -v

test:
	@echo ">> running tests"
	@$(GO) test -short ./...

format:
	@echo ">> formatting code"
	@$(GO) fmt ./...

vet:
	@echo ">> vetting code"
	@$(GO) vet ./...

cross: promu
	@echo ">> crossbuilding binaries"
	@$(PROMU) crossbuild

tarballs: promu
	@echo ">> creating release tarballs"
	@$(PROMU) crossbuild tarballs

build: promu
	@echo ">> building binaries"
	@$(PROMU) build -v --prefix $(PREFIX)

crossbuild: promu
	@echo ">> building binaries"
	@$(PROMU) crossbuild -v

tarball: promu
	@echo ">> building release tarball"
	@$(PROMU) tarball $(BIN_DIR)

docker: cross
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

docker-150:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):v1.5.0" .

docker-publish:
	@echo ">> publishing docker image"
	@docker push "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME)"

docker-tag-latest:
	@docker tag "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):latest"

promu:
	@GO111MODULE=off \
		    GOOS=$(shell uname -s | tr A-Z a-z) \
	        GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
	        $(GO) get -u github.com/prometheus/promu


.PHONY: all style dep format build test vet tarball docker docker-publish docker-tag-latest promu


run:
	go run *.go
