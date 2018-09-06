# Need CGO_ENABLED=1 for go-sqlite3
GO                      ?= CGO_ENABLED=1 go
GOFMT                   ?= $(GO)fmt
FIRST_GOPATH            := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU                   := $(FIRST_GOPATH)/bin/promu

GOMETALINTER_BINARY     := CGO_ENABLED=1 $(FIRST_GOPATH)/bin/gometalinter
DEP_BINARY              := $(FIRST_GOPATH)/bin/dep

PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)
DOCKER_IMAGE_NAME       ?= telly
DOCKER_IMAGE_TAG        ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))


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
	@$(GOFMT) .

vet:
	@echo ">> vetting code"
	@$(GO) vet ./...

build: promu
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX)

tarball: promu
	@echo ">> building release tarball"
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

docker-publish:
	@echo ">> publishing docker image"
	@docker push "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME)"

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
	        GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
	        $(GO) get -u github.com/prometheus/promu


.PHONY: all style dep format build test vet tarball docker docker-publish promu


run:
	go run *.go
