SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

##@ Invariants

.PHONY: lint-leak
lint-leak: ## Enforce INV-1 and INV-6: no backend or cluster-selection identifier reaches a user
	go run ./cmd/lint-leak -root .

##@ Help

.PHONY: help
help: ## Print this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
