# Runs the agentgateway compatibility test on kind. See test/extmcp/.

include test/extmcp/versions.env

SHELL := bash
.SHELLFLAGS := -eo pipefail -c

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

.PHONY: test clean tools cluster gateway image deploy e2e

test: tools cluster gateway image deploy e2e

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
e2e:
	cd $(E2E) && go test -tags $(E2E_TAGS) -count=1 -v ./... | tee e2e.log
	@grep -q -- '^--- PASS: TestE2E ' $(E2E)/e2e.log || { echo "FAIL: TestE2E did not run and pass"; exit 1; }

clean:
	kind delete cluster --name $(CLUSTER)
