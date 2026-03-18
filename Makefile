ifeq ($(OS),Windows_NT)
EXEEXT := .exe
else
SHELL := /bin/sh
EXEEXT :=
endif

ifeq ($(OS),Windows_NT)
define MKDIR_P
powershell -NoProfile -Command "New-Item -ItemType Directory -Force '$(1)' | Out-Null"
endef

define RM_RF
powershell -NoProfile -Command "if (Test-Path '$(1)') { Remove-Item -Recurse -Force '$(1)' }"
endef
else
define MKDIR_P
mkdir -p $(1)
endef

define RM_RF
rm -rf $(1)
endef
endif

include versions/toolchain.env

IMAGE ?= ghcr.io/mb3r-lab/coroot-graft:dev
HELM_BIN ?= helm
DIST_DIR ?= dist
APP_VERSION ?= 0.1.2
ifeq ($(OS),Windows_NT)
GIT_COMMIT ?= $(strip $(shell powershell -NoProfile -Command "$$commit = git rev-parse HEAD 2>$$null; if ($$LASTEXITCODE -eq 0 -and $$commit) { $$commit } else { 'unknown' }"))
BUILD_DATE ?= $(strip $(shell powershell -NoProfile -Command "(Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')"))
else
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
endif

.PHONY: help build test lint docker-build chart-lint chart-template clean

help:
	@echo "Targets:"
	@echo "  build          Build the coroot-graft CLI locally"
	@echo "  test           Run Go tests"
	@echo "  lint           Run go vet"
	@echo "  docker-build   Build the production image with pinned Bering and Sheaft"
	@echo "  chart-lint     Lint charts/coroot-graft"
	@echo "  chart-template Render charts/coroot-graft with default values"
	@echo "  clean          Remove bin and dist"

build:
	$(call MKDIR_P,bin)
	go build -o bin/coroot-graft$(EXEEXT) ./cmd/coroot-graft

test:
	go test ./...

lint:
	go vet ./...

docker-build:
	docker build -f build/Dockerfile -t $(IMAGE) \
		--build-arg VERSION=$(APP_VERSION) \
		--build-arg COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg BERING_REPOSITORY=$(BERING_REPOSITORY) \
		--build-arg BERING_REF=$(BERING_REF) \
		--build-arg SHEAFT_REPOSITORY=$(SHEAFT_REPOSITORY) \
		--build-arg SHEAFT_REF=$(SHEAFT_REF) \
		.

chart-lint:
	$(HELM_BIN) lint charts/coroot-graft

chart-template:
	$(HELM_BIN) template coroot-graft charts/coroot-graft

clean:
	$(call RM_RF,bin)
	$(call RM_RF,$(DIST_DIR))
