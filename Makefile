# Checks for the repo. `make check` runs lint and unit tests. `make e2e` runs the
# agentgateway compatibility test on kind. See test/extmcp/ and AGENTS.md.

include versions.env
include test/extmcp/versions.env

SHELL := bash
.SHELLFLAGS := -eo pipefail -c

# Go modules that test-go, lint-go and vuln loop over.
GO_MODULES := test/extmcp
PY_PROJECT := agents/azure-ops

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
ACTIONLINT := go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
GITLEAKS := go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION)

E2E := test/extmcp
CLUSTER := agent-gate-extmcp
NS := agent-gate-test
IMAGE := agent-gate/extmcp-stubs:dev
CHARTS := oci://ghcr.io/agentgateway/charts
GATEWAY_API := https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml
E2E_TAGS ?= e2e

# The test owns its kubeconfig. It never reads or changes ~/.kube.
export KUBECONFIG := $(CURDIR)/$(E2E)/.kube/config
export KUBECACHEDIR := $(CURDIR)/$(E2E)/.kube/cache

.PHONY: check lint test test-go test-python lint-go lint-python lint-actions vuln secrets \
	e2e clean tools cluster gateway image deploy e2e-run

check: lint test

lint: lint-go lint-python lint-actions

test: test-go test-python

test-go:
	@for m in $(GO_MODULES); do \
		echo "go test $$m"; (cd $$m && go test -count=1 ./...) || exit 1; \
	done

lint-go:
	@for m in $(GO_MODULES); do \
		echo "golangci-lint $$m"; \
		(cd $$m && $(GOLANGCI_LINT) run --config $(CURDIR)/.golangci.yml ./...) || exit 1; \
	done

test-python:
	cd $(PY_PROJECT) && uv run --locked pytest

lint-python:
	cd $(PY_PROJECT) && uv run --locked ruff check && uv run --locked ruff format --check

lint-actions:
	$(ACTIONLINT) -shellcheck= -pyflakes=

vuln:
	@for m in $(GO_MODULES); do \
		echo "govulncheck $$m"; (cd $$m && $(GOVULNCHECK) ./...) || exit 1; \
	done

# HEAD only: by default gitleaks scans all refs, and one other branch would fail each scan.
secrets:
	$(GITLEAKS) git --redact --verbose --log-opts="--full-history HEAD"

e2e: tools cluster gateway image deploy e2e-run

tools:
	@for t in go docker kind kubectl helm; do \
		command -v $$t >/dev/null || { echo "missing tool: $$t"; exit 1; }; \
	done

cluster:
	kind delete cluster --name $(CLUSTER)
	kind create cluster --name $(CLUSTER) --image $(KIND_NODE_IMAGE) --config $(E2E)/deploy/kind.yaml --wait 120s

gateway:
	kubectl apply --server-side -f $(GATEWAY_API)
	helm upgrade --install agentgateway-crds $(CHARTS)/agentgateway-crds --version $(AGENTGATEWAY_VERSION) \
		--namespace agentgateway-system --create-namespace --wait
	helm upgrade --install agentgateway $(CHARTS)/agentgateway --version $(AGENTGATEWAY_VERSION) \
		--namespace agentgateway-system --wait
	kubectl -n agentgateway-system wait deployment --all --for=condition=Available --timeout=180s
	kubectl wait gatewayclass/agentgateway --for=condition=Accepted --timeout=120s
	kubectl apply -f $(E2E)/deploy/gateway.yaml
	kubectl -n $(NS) wait gateway/gateway --for=condition=Programmed --timeout=180s

image:
	docker build --tag $(IMAGE) $(E2E)
	kind load docker-image $(IMAGE) --name $(CLUSTER)

deploy:
	kubectl apply -f $(E2E)/deploy/stubs.yaml -f $(E2E)/deploy/routing.yaml
	kubectl -n $(NS) wait deployment --all --for=condition=Available --timeout=180s
	kubectl -n $(NS) wait agentgatewaypolicy --all --timeout=60s \
		--for=jsonpath='{.status.ancestors[0].conditions[?(@.type=="Accepted")].status}'=True

# The grep fails the target when TestE2E did not run, for example with a wrong build tag.
e2e-run:
	cd $(E2E) && go test -tags $(E2E_TAGS) -count=1 -v ./... | tee e2e.log
	@grep -q -- '^--- PASS: TestE2E ' $(E2E)/e2e.log || { echo "FAIL: TestE2E did not run and pass"; exit 1; }

clean:
	kind delete cluster --name $(CLUSTER)
