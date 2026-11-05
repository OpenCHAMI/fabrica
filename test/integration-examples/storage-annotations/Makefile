# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Integration test set for examples/12-storage-annotations.
#
# Steps (each can be run on its own):
#   make generate    scaffold + generate the service under build/ using
#                    `fabrica init` and `fabrica add resources resource-defs`
#   make up          build a linux server binary and start it with `fabrun start`
#   make test-only   run the Go (testify) tests against the running service
#   make down        stop the service with `fabrun stop`
#
#   make test        generate, up, test-only, then always down

.DEFAULT_GOAL := help

GO ?= go
GOEXE := $(shell $(GO) env GOEXE)
export GOTOOLCHAIN ?= auto

EXAMPLE_DIR  := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
FABRICA_ROOT := $(abspath $(EXAMPLE_DIR)/../../..)
TESTS_ROOT   := $(abspath $(EXAMPLE_DIR)/..)
FABRICA      := $(FABRICA_ROOT)/bin/fabrica$(GOEXE)
FABRUN       := $(FABRICA_ROOT)/bin/fabrun$(GOEXE)

# Generated service settings.
DB      ?= sqlite
SERVICE ?= user-service
MODULE  ?= github.com/openchami/$(SERVICE)
GROUP   ?= storage-annotations.openchami.org
BUILD_DIR   := $(EXAMPLE_DIR)/build
SERVICE_DIR := $(BUILD_DIR)/$(SERVICE)
# Relative so it resolves both on the host and inside the build container. The
# service's go directive is raised to fabrica's so the replaced module builds.
FABRICA_REPLACE := ../../../../..
FABRICA_GO_VERSION := $(shell awk '/^go / {print $$2; exit}' $(FABRICA_ROOT)/go.mod)

# fabrun settings. NAME is the container name (and sqlite db file name).
NAME    ?= fabrica-example-storage-annotations
PORT    ?= 18080
PG_PORT ?= 15433
BASE_URL ?= http://localhost:$(PORT)

# The server is built for linux inside a golang container because the sqlite
# driver needs cgo, which rules out simple cross-compilation from the host.
# Set BUILD_IN_DOCKER=0 to build with the host toolchain instead (linux hosts).
BUILD_IN_DOCKER ?= 1
GO_IMAGE ?= golang:$(shell echo $(FABRICA_GO_VERSION) | cut -d. -f1,2)
CONTAINER_BUILD_DIR := /fabrica/$(patsubst $(FABRICA_ROOT)/%,%,$(BUILD_DIR))

# docker.exe on Windows needs native paths for bind mounts.
hostpath = $(shell cygpath -m '$(1)' 2>/dev/null || echo '$(1)')
DOCKER_USER := $(if $(filter Linux Darwin,$(shell uname -s)),--user $(shell id -u):$(shell id -g))

TEST_FLAGS ?= -count=1 -v

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Build bin/fabrica and bin/fabrun from this checkout
	$(MAKE) -C $(FABRICA_ROOT) build build-fabrun

.PHONY: generate
generate: tools ## Generate the example service under build/ (DB=sqlite|postgres)
	rm -rf $(SERVICE_DIR)
	mkdir -p $(BUILD_DIR)
	$(FABRICA) init $(SERVICE) \
		--module $(MODULE) \
		--group $(GROUP) \
		--storage-version v1 \
		--storage-type ent \
		--db $(DB) \
		--dir $(call hostpath,$(BUILD_DIR))
	cd $(SERVICE_DIR) && $(GO) mod edit \
		-replace github.com/openchami/fabrica=$(FABRICA_REPLACE) \
		-go $(FABRICA_GO_VERSION)
	cd $(SERVICE_DIR) && $(FABRICA) add resources $(call hostpath,$(EXAMPLE_DIR)/resource-defs)
	cd $(SERVICE_DIR) && $(FABRICA) generate
	cd $(SERVICE_DIR) && GOFLAGS=-mod=mod $(GO) generate ./...
	cd $(SERVICE_DIR) && $(GO) mod tidy
	cd $(SERVICE_DIR) && $(GO) build ./...

.PHONY: build-server
build-server: ## Build the linux server binary fabrun runs (build/<service>/bin/<service>)
	@test -f $(SERVICE_DIR)/go.mod || { echo "no generated service at $(SERVICE_DIR); run 'make generate' first"; exit 1; }
	mkdir -p $(SERVICE_DIR)/bin
ifeq ($(BUILD_IN_DOCKER),1)
	mkdir -p $(BUILD_DIR)/.cache/go-build
	docker run --rm $(DOCKER_USER) \
		-e HOME=/tmp -e GOTOOLCHAIN=auto \
		-e GOMODCACHE=/gomodcache \
		-e GOCACHE=$(CONTAINER_BUILD_DIR)/.cache/go-build \
		-v $(call hostpath,$(FABRICA_ROOT)):/fabrica \
		-v $(call hostpath,$(shell $(GO) env GOMODCACHE)):/gomodcache \
		-w $(CONTAINER_BUILD_DIR)/$(SERVICE) \
		$(GO_IMAGE) go build -o bin/$(SERVICE) ./cmd/server
else
	cd $(SERVICE_DIR) && CGO_ENABLED=1 GOOS=linux $(GO) build -o bin/$(SERVICE) ./cmd/server
endif

.PHONY: up
up: build-server ## Start the service with fabrun (PORT=$(PORT))
	$(FABRUN) start --service $(call hostpath,$(SERVICE_DIR)) --name $(NAME) --port $(PORT) --pg-port $(PG_PORT)
	@echo "service is up at $(BASE_URL)"

.PHONY: test-only
test-only: ## Run the tests against an already running service (see 'make up')
	cd $(TESTS_ROOT) && \
		FABRICA_EXAMPLE_URL=$(BASE_URL) \
		FABRICA_EXAMPLE_NAME=$(NAME) \
		FABRICA_EXAMPLE_DB=$(DB) \
		FABRUN=$(call hostpath,$(FABRUN)) \
		$(GO) test $(TEST_FLAGS) ./storage-annotations/...

.PHONY: down
down: ## Stop the service with fabrun (removes its data volume)
	-$(FABRUN) stop --name $(NAME)

.PHONY: test
test: generate ## Run everything: generate, up, test-only, down
	$(MAKE) up
	status=0; $(MAKE) test-only || status=$$?; $(MAKE) down; exit $$status

.PHONY: clean
clean: down ## Stop the service and remove build/
	rm -rf $(BUILD_DIR)
