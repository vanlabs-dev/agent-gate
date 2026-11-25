# Checks for the repo. `make check` runs lint and unit tests. `make e2e` runs the
# agentgateway compatibility test on kind, `make e2e-gate` the gate end-to-end
# test on its own kind cluster. See test/extmcp/, gate/ and AGENTS.md.

include versions.env
include test/extmcp/versions.env
include gate/versions.env

SHELL := bash
.SHELLFLAGS := -eo pipefail -c

# Go modules that test-go, lint-go and vuln loop over.
GO_MODULES := test/extmcp gate
PY_PROJECT := agents/azure-ops

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
ACTIONLINT := go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
GITLEAKS := go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION)

# The kind steps (cluster, gateway, image) read E2E, CLUSTER, NS and IMAGES.
# e2e-gate sets its own values, which its prerequisites inherit.
E2E := test/extmcp
CLUSTER := agent-gate-extmcp
NS := agent-gate-test
STUBS_IMAGE := agent-gate/extmcp-stubs:dev
GATE_IMAGE := agent-gate/gate:dev
# Each item is <tag>=<build context>.
IMAGES := $(STUBS_IMAGE)=test/extmcp
KIND_CONFIG := test/extmcp/deploy/kind.yaml
CHARTS := oci://ghcr.io/agentgateway/charts
GATEWAY_API := https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml
E2E_TAGS ?= e2e

# Each test owns its kubeconfig. It never reads or changes ~/.kube.
export KUBECONFIG = $(CURDIR)/$(E2E)/.kube/config
export KUBECACHEDIR = $(CURDIR)/$(E2E)/.kube/cache

e2e-gate e2e-gate-deploy e2e-gate-run: E2E := gate
e2e-gate e2e-gate-deploy e2e-gate-run: CLUSTER := agent-gate-e2e
e2e-gate e2e-gate-deploy e2e-gate-run: NS := agent-gate-e2e
e2e-gate e2e-gate-deploy e2e-gate-run: IMAGES := $(STUBS_IMAGE)=test/extmcp $(GATE_IMAGE)=gate

.PHONY: check lint test test-go test-python lint-go lint-python lint-actions vuln secrets \
	e2e e2e-gate clean tools cluster gateway image deploy e2e-run e2e-gate-deploy e2e-gate-run

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
	kind create cluster --name $(CLUSTER) --image $(KIND_NODE_IMAGE) --config $(KIND_CONFIG) --wait 120s

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
	@for i in $(IMAGES); do \
		tag=$${i%%=*}; dir=$${i#*=}; echo "docker build $$tag $$dir"; \
		docker build --tag $$tag $$dir && kind load docker-image $$tag --name $(CLUSTER) || exit 1; \
	done

deploy:
	kubectl apply -f $(E2E)/deploy/stubs.yaml -f $(E2E)/deploy/routing.yaml
	kubectl -n $(NS) wait deployment --all --for=condition=Available --timeout=180s
	kubectl -n $(NS) wait agentgatewaypolicy --all --timeout=60s \
		--for=jsonpath='{.status.ancestors[0].conditions[?(@.type=="Accepted")].status}'=True

# The grep fails the target when TestE2E did not run, for example with a wrong build tag.
e2e-run:
	cd $(E2E) && go test -tags $(E2E_TAGS) -count=1 -v ./... | tee e2e.log
	@grep -q -- '^--- PASS: TestE2E ' $(E2E)/e2e.log || { echo "FAIL: TestE2E did not run and pass"; exit 1; }

# The gate end-to-end test. Dex and Postgres start first, because the gate
# reads the Dex discovery document and runs its migrations at start.
e2e-gate: tools cluster gateway image e2e-gate-deploy e2e-gate-run

e2e-gate-deploy:
	sed -e 's|DEX_IMAGE|$(DEX_IMAGE)|' gate/deploy/dex.yaml | kubectl apply -f -
	sed -e 's|POSTGRES_IMAGE|$(POSTGRES_IMAGE)|' gate/deploy/postgres.yaml | kubectl apply -f -
	kubectl -n $(NS) wait deployment/dex deployment/postgres --for=condition=Available --timeout=180s
	kubectl -n $(NS) create configmap agent-gate-policy --from-file=gate/deploy/policy \
		--dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f gate/deploy/gate.yaml -f gate/deploy/routing.yaml
	kubectl -n $(NS) wait deployment --all --for=condition=Available --timeout=180s
	kubectl -n $(NS) wait agentgatewaypolicy --all --timeout=60s \
		--for=jsonpath='{.status.ancestors[0].conditions[?(@.type=="Accepted")].status}'=True

# The Postgres store tests run on their own database (agent_gate_pgtest) of
# the cluster's Postgres, through a port-forward. The grep fails the target
# when TestE2E did not run, for example with a wrong build tag.
e2e-gate-run:
	@kubectl -n $(NS) port-forward service/postgres 15433:5432 >/dev/null 2>&1 & pf=$$!; \
		trap "kill $$pf" EXIT; \
		for i in $$(seq 30); do (exec 3<>/dev/tcp/127.0.0.1/15433) 2>/dev/null && break; sleep 1; done; \
		cd gate && AGENT_GATE_PGTEST_DSN='postgres://postgres@127.0.0.1:15433/postgres?sslmode=disable' \
			go test -tags pgtest -count=1 -v ./internal/postgres/ | tee pgtest.log
	cd gate && go test -tags $(E2E_TAGS) -count=1 -v ./e2e/ | tee e2e.log
	@grep -q -- '^--- PASS: TestE2E ' gate/e2e.log || { echo "FAIL: TestE2E did not run and pass"; exit 1; }

clean:
	kind delete cluster --name agent-gate-extmcp
	kind delete cluster --name agent-gate-e2e
