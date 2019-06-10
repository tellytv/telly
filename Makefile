GO    := go
GOPATH ?= $(HOME)/go
PROMU := $(GOPATH)/bin/promu
CILINT := $(GOPATH)/bin/golangci-lint

PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)
DOCKER_IMAGE_NAME       ?= telly
DOCKER_IMAGE_TAG 		?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

all: format build test

style:
	@echo ">> checking code style"
	@$(GO) get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@$(CILINT) run ./...

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
	@$(PROMU) build --prefix $(PREFIX)

tarball: promu
	@echo ">> building release tarball"
	@$(PROMU) tarball $(BIN_DIR)

docker: cross
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:
	@$(GO) get -u github.com/prometheus/promu

.PHONY: all style format build test vet tarball docker promu