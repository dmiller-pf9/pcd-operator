SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# ---- pinned tool versions -------------------------------------------------
# Bump deliberately, never implicitly. `go install ...@latest` in a Makefile
# makes CI non-reproducible.
CONTROLLER_TOOLS_VERSION ?= v0.22.0
GOLANGCI_LINT_VERSION    ?= v2.13.2
SETUP_ENVTEST_VERSION    ?= v0.25.1
# The control-plane version envtest downloads. Confirmed against the available
# envtest releases when milestone 010 adds the first envtest test.
ENVTEST_K8S_VERSION      ?= 1.34.1

LOCALBIN         ?= $(shell pwd)/bin
CONTROLLER_GEN   ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT    ?= $(LOCALBIN)/golangci-lint
SETUP_ENVTEST    ?= $(LOCALBIN)/setup-envtest

# T is the -run pattern for `make test-one`.
T ?=

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

##@ Code generation

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Regenerate CRD manifests from api/
	@if [ -z "$$(find api -name '*.go' 2>/dev/null)" ]; then \
		echo "manifests: api/ has no Go types yet, nothing to generate"; \
	else \
		$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases; \
	fi

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Regenerate deepcopy functions from api/
	@if [ -z "$$(find api -name '*.go' 2>/dev/null)" ]; then \
		echo "generate: api/ has no Go types yet, nothing to generate"; \
	else \
		$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."; \
	fi

##@ Test

.PHONY: test
test: ## Unit tests only, no cluster needed
	go test ./... -count=1

.PHONY: test-one
test-one: ## Run a single test: make test-one T=TestName
	@test -n "$(T)" || { echo "usage: make test-one T=TestName"; exit 2; }
	go test ./... -run '$(T)' -count=1 -v

.PHONY: test-envtest
test-envtest: $(SETUP_ENVTEST) ## Controller tests against envtest
	KUBEBUILDER_ASSETS="$$($(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./... -count=1 -tags envtest

##@ Lint

.PHONY: lint
lint: $(GOLANGCI_LINT) ## golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: lint-leak
lint-leak: ## Enforce INV-1 and INV-6: no backend or cluster-selection identifier reaches a user
	go run ./cmd/lint-leak -root .

##@ Verify

.PHONY: verify
verify: manifests generate check-generated lint lint-leak test ## Everything CI runs

.PHONY: check-generated
check-generated: ## Fail if manifests/generate produced uncommitted changes
	@if ! git diff --quiet -- api config; then \
		echo "ERROR: generated files are out of date. Run 'make manifests generate' and commit the result."; \
		git --no-pager diff --stat -- api config; \
		exit 1; \
	fi

##@ Tools

$(CONTROLLER_GEN): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

$(GOLANGCI_LINT): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(SETUP_ENVTEST): | $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)

##@ Help

.PHONY: help
help: ## Print this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
