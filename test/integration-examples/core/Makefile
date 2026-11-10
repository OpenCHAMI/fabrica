# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Integration test set that generates the same two-resource service (Node and
# User, see resource-defs/) with each storage backend and runs one shared test
# suite against all three:
#
#   file      fabrica file storage           (--storage-type file)
#   sqlite    ent storage, sqlite driver     (--storage-type ent --db sqlite)
#   postgres  ent storage, postgres driver   (--storage-type ent --db postgres)
#
# Every step exists for all backends at once and per backend (BACKEND is one of
# file, sqlite, postgres):
#
#   make generate            make generate-BACKEND   fabrica init + add resources + generate
#   make up                  make up-BACKEND         build linux server, `fabrun start`
#   make test-only           make test-only-BACKEND  run the Go tests against a running service
#   make down                make down-BACKEND       `fabrun stop` (removes data volumes)
#   make test                make test-BACKEND       generate, up, test-only, then always down
#
# `make test` runs every backend even when an earlier one fails and exits
# non-zero if any failed.
#
# The file and postgres servers are pure Go and are cross-compiled with
# CGO_ENABLED=0. The sqlite driver (mattn/go-sqlite3) needs cgo, so its server
# is built inside a golang:<ver>-bookworm container by default (matching the
# debian:bookworm-slim image fabrun runs it in). On a linux host with a C
# compiler and a glibc no newer than bookworm's, BUILD_IN_DOCKER=0 builds it
# with the host toolchain instead.

.DEFAULT_GOAL := help

GO ?= go
GOEXE := $(shell $(GO) env GOEXE)
export GOTOOLCHAIN ?= auto

EXAMPLE_DIR  := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
EXAMPLE      := $(notdir $(EXAMPLE_DIR))
FABRICA_ROOT := $(abspath $(EXAMPLE_DIR)/../../..)
TESTS_ROOT   := $(abspath $(EXAMPLE_DIR)/..)
FABRICA      := $(FABRICA_ROOT)/bin/fabrica$(GOEXE)
FABRUN       := $(FABRICA_ROOT)/bin/fabrun$(GOEXE)

BACKENDS := file sqlite postgres

# Generated service settings. Each backend is generated into
# build/$(SERVICE)-<backend>.
SERVICE ?= node-user-service
MODULE  ?= github.com/openchami/$(SERVICE)
GROUP   ?= core.openchami.org
BUILD_DIR := $(EXAMPLE_DIR)/build
# Relative so it resolves both on the host and inside the build container
# (build/<service>-<backend> is five levels below the fabrica root). The
# service's go directive is raised to fabrica's so the replaced module builds.
FABRICA_REPLACE := ../../../../..
FABRICA_GO_VERSION := $(shell awk '/^go / {print $$2; exit}' $(FABRICA_ROOT)/go.mod)

STORAGE_ARGS_file     := --storage-type file
STORAGE_ARGS_sqlite   := --storage-type ent --db sqlite
STORAGE_ARGS_postgres := --storage-type ent --db postgres

CGO_file     := 0
CGO_sqlite   := 1
CGO_postgres := 0

# fabrun settings. NAME_<backend> is the container name (postgres also gets a
# <name>-postgres container). Host ports are distinct so all three services can
# run at the same time.
NAME_PREFIX   ?= fabrica-example-$(EXAMPLE)
NAME_file     ?= $(NAME_PREFIX)-file
NAME_sqlite   ?= $(NAME_PREFIX)-sqlite
NAME_postgres ?= $(NAME_PREFIX)-postgres
PORT_file     ?= 18082
PORT_sqlite   ?= 18083
PORT_postgres ?= 18084
# Host port for the postgres database container.
PG_PORT       ?= 15433

FABRUN_ARGS_postgres := --pg-port $(PG_PORT)

# Build the cgo (sqlite) server in a container. Set to 0 on a suitable linux host.
BUILD_IN_DOCKER ?= 1
GO_IMAGE ?= golang:$(FABRICA_GO_VERSION)-bookworm
DOCKER   ?= docker

# docker.exe on Windows needs native paths for bind mounts.
hostpath = $(shell cygpath -m '$(1)' 2>/dev/null || echo '$(1)')

TEST_FLAGS ?= -count=1 -v

.PHONY: help
help: ## Display this help screen
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Per-backend targets (BACKEND is one of: $(BACKENDS)):"
	@printf "\033[36m%-22s\033[0m %s\n" \
		generate-BACKEND     "Generate one backend's service under build/<service>-BACKEND/" \
		build-server-BACKEND "Build one backend's linux server binary" \
		up-BACKEND           "Start one backend's service with fabrun" \
		test-only-BACKEND    "Run the tests against one backend's running service" \
		down-BACKEND         "Stop one backend's service (removes its data volume)" \
		test-BACKEND         "generate, up, test-only, then always down for one backend"

.PHONY: tools
tools: ## Build bin/fabrica and bin/fabrun from this checkout
	$(MAKE) -C $(FABRICA_ROOT) build build-fabrun

# backend_rules generates the per-backend targets for backend $(1).
define backend_rules
SERVICE_DIR_$(1) := $(BUILD_DIR)/$(SERVICE)-$(1)

.PHONY: generate-$(1)
generate-$(1): tools
	rm -rf $$(SERVICE_DIR_$(1)) $(BUILD_DIR)/.tmp-$(1)
	mkdir -p $(BUILD_DIR)/.tmp-$(1)
	$(FABRICA) init $(SERVICE) \
		--module $(MODULE) \
		--group $(GROUP) \
		--storage-version v1 \
		$(STORAGE_ARGS_$(1)) \
		--dir $$(call hostpath,$(BUILD_DIR)/.tmp-$(1))
	mv $(BUILD_DIR)/.tmp-$(1)/$(SERVICE) $$(SERVICE_DIR_$(1))
	rmdir $(BUILD_DIR)/.tmp-$(1)
	cd $$(SERVICE_DIR_$(1)) && $(GO) mod edit \
		-replace github.com/openchami/fabrica=$(FABRICA_REPLACE) \
		-go $(FABRICA_GO_VERSION)
	cd $$(SERVICE_DIR_$(1)) && $(FABRICA) add resources $$(call hostpath,$(EXAMPLE_DIR)/resource-defs)
	cd $$(SERVICE_DIR_$(1)) && $(FABRICA) generate
	cd $$(SERVICE_DIR_$(1)) && $(GO) mod tidy
	cd $$(SERVICE_DIR_$(1)) && $(GO) build ./...

.PHONY: build-server-$(1)
build-server-$(1):
	@test -f $$(SERVICE_DIR_$(1))/go.mod || { echo "no generated service at $$(SERVICE_DIR_$(1)); run 'make generate-$(1)' first"; exit 1; }
	mkdir -p $$(SERVICE_DIR_$(1))/bin
ifeq ($(CGO_$(1))$(BUILD_IN_DOCKER),11)
	$(DOCKER) run --rm \
		-v $$(call hostpath,$(FABRICA_ROOT)):/src \
		-v fabrica-integration-gomod:/go/pkg/mod \
		-v fabrica-integration-gocache:/root/.cache/go-build \
		-w /src/$$(patsubst $(FABRICA_ROOT)/%,%,$$(SERVICE_DIR_$(1))) \
		-e CGO_ENABLED=1 -e GOOS=linux -e GOTOOLCHAIN=auto \
		$(GO_IMAGE) \
		sh -c 'go build -o bin/$(SERVICE) ./cmd/server && chown $$(shell id -u):$$(shell id -g) bin/$(SERVICE)'
else
	cd $$(SERVICE_DIR_$(1)) && CGO_ENABLED=$(CGO_$(1)) GOOS=linux $(GO) build -o bin/$(SERVICE) ./cmd/server
endif

.PHONY: up-$(1)
up-$(1): build-server-$(1)
	$(FABRUN) start --service $$(call hostpath,$$(SERVICE_DIR_$(1))) --name $(NAME_$(1)) --port $(PORT_$(1)) --db $(1) $(FABRUN_ARGS_$(1))
	@echo "$(1) service is up at http://localhost:$(PORT_$(1))"

.PHONY: test-only-$(1)
test-only-$(1):
	cd $(TESTS_ROOT) && \
		FABRICA_EXAMPLE_URL=http://localhost:$(PORT_$(1)) \
		FABRICA_EXAMPLE_NAME=$(NAME_$(1)) \
		FABRICA_EXAMPLE_BACKEND=$(1) \
		$(GO) test $(TEST_FLAGS) ./$(EXAMPLE)/...

.PHONY: down-$(1)
down-$(1):
	-$(FABRUN) stop --name $(NAME_$(1))

.PHONY: test-$(1)
test-$(1): generate-$(1)
	$(MAKE) up-$(1)
	status=0; $(MAKE) test-only-$(1) || status=$$$$?; $(MAKE) down-$(1); exit $$$$status
endef

$(foreach b,$(BACKENDS),$(eval $(call backend_rules,$(b))))

.PHONY: generate
generate: $(addprefix generate-,$(BACKENDS)) ## Generate all backends' services

.PHONY: up
up: $(addprefix up-,$(BACKENDS)) ## Start all backends' services

.PHONY: test-only
test-only: ## Run the tests against all running services
	status=0; for b in $(BACKENDS); do $(MAKE) test-only-$$b || status=1; done; exit $$status

.PHONY: down
down: $(addprefix down-,$(BACKENDS)) ## Stop all backends' services

.PHONY: test
test: ## Run test-<backend> for every backend (continues past failures)
	status=0; for b in $(BACKENDS); do \
		echo "== storage backend: $$b =="; \
		$(MAKE) test-$$b || status=1; \
	done; exit $$status

.PHONY: clean
clean: down ## Stop all services and remove build/
	rm -rf $(BUILD_DIR)
