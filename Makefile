# ==============================================================================
# Octpex Biomimetic Multi-Agent Local & Kubernetes Development Loop
# ==============================================================================
# Note on Container Registries:
# When developing with local Kubernetes (Kind, Docker Desktop Kind nodes, etc.),
# images built by host Docker ("dockerd") reside in the host Docker storage pool.
# K8s nodes run their own containerd CRI daemon inside the node container,
# which strictly looks into the "k8s.io" containerd namespace.
#
# Without pushing to an external registry or loading the image into the node's
# containerd runtime, Kubernetes will fail with ImagePullBackOff when pulling
# local images. The 'load' target streams the image directly into the node's
# containerd without intermediate tar files on disk.
# ==============================================================================

export CGO_ENABLED ?= 0

IMAGE     ?= octpex/kernel:0.5.0
NODE      ?= desktop-control-plane
NAMESPACE ?= octpex-system
BIN_DIR   ?= bin
BIN_NAME  ?= octpex

# Run configuration parameters (can be overridden via CLI, e.g. make run PROFILE=cluster)
PROFILE   ?= local
CONFIG    ?= configs/$(PROFILE).yaml
PORT      ?= 8080
ARGS      ?=

ifeq ($(OS),Windows_NT)
    EXEC_EXT := .exe
else
    EXEC_EXT :=
endif

TARGET_BIN := $(BIN_DIR)/$(BIN_NAME)$(EXEC_EXT)

.PHONY: help build build-bin docker-build run load deploy dev status logs

help:
	@echo "Octpex Development Targets:"
	@echo "  make build        - Compile host native static binary (CGO_ENABLED=0) into ./bin/"
	@echo "  make run          - Run daemon (default: PROFILE=local PORT=8080; e.g. make run PROFILE=cluster)"
	@echo "  make docker-build - Build kernel Docker container image locally"
	@echo "  make load         - Stream container image into Kind/K8s node containerd"
	@echo "  make deploy       - Rollout restart entry & worker deployments in K8s"
	@echo "  make dev          - Run K8s loop: docker-build -> load -> deploy"
	@echo "  make status       - Inspect running pods in $(NAMESPACE)"
	@echo "  make logs         - Tail logs of octpex-entry"

build: build-bin

build-bin:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(TARGET_BIN) ./cmd/octpex

run: build-bin
	$(TARGET_BIN) run -profile=$(PROFILE) -config=$(CONFIG) -port=$(PORT) $(ARGS)

docker-build:
	docker buildx build --load -t $(IMAGE) -f deploy/docker/Dockerfile .

load: docker-build
	docker save $(IMAGE) | docker exec -i $(NODE) ctr -n k8s.io images import -

deploy:
	kubectl rollout restart deployment/octpex-entry -n $(NAMESPACE)
	kubectl rollout restart deployment/octpex-worker -n $(NAMESPACE)

dev: load deploy

status:
	kubectl get pods -n $(NAMESPACE) -o wide

logs:
	kubectl logs -n $(NAMESPACE) -l app=octpex-entry -f --tail=100
