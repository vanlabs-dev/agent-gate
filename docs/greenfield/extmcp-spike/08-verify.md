# Verify: extmcp-spike

- Date: 2026-11-16
- Command, from the repo root: `bash ~/.claude/skills/greenfield/scripts/verify.sh extmcp-spike .`
- Exit code: 0
- Result: PASS
- Where: the development machine. The CI run is not part of this output.
- The home directory in the two path lines is written as `~`. The output is otherwise unchanged.

```text
=== greenfield verify: extmcp-spike ===
Project root: ~/src/github/vanlabs-dev/agent-gate
Artifact dir: ~/src/github/vanlabs-dev/agent-gate/docs/greenfield/extmcp-spike

Checking greenfield artifacts...
  OK  01-brainstorm.md
  OK  07-review.md
  note  09-ship.md not present yet

Checking OpenSpec change...
  OK  openspec/changes/extmcp-spike

Running project checks (if present)...
kind delete cluster --name agent-gate-extmcp
Deleting cluster "agent-gate-extmcp" ...
kind create cluster --name agent-gate-extmcp --image kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5 --config test/extmcp/deploy/kind.yaml --wait 120s
Creating cluster "agent-gate-extmcp" ...
 • Ensuring node image (kindest/node:v1.37.0) 🖼️  ...
 ✓ Ensuring node image (kindest/node:v1.37.0) 🖼️
 • Preparing nodes 📦   ...
 ✓ Preparing nodes 📦 
 • Writing configuration 📜  ...
 ✓ Writing configuration 📜
 • Starting control-plane 🕹️  ...
 ✓ Starting control-plane 🕹️
 • Installing CNI 🔌  ...
 ✓ Installing CNI 🔌
 • Installing StorageClass 💾  ...
 ✓ Installing StorageClass 💾
 • Waiting ≤ 2m0s for control-plane = Ready ⏳  ...
 ✓ Waiting ≤ 2m0s for control-plane = Ready ⏳
 • Ready after 16s 💚
Set kubectl context to "kind-agent-gate-extmcp"
You can now use your cluster with:

kubectl cluster-info --context kind-agent-gate-extmcp

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.7.0-rc.3/standard-install.yaml
customresourcedefinition.apiextensions.k8s.io/backendtlspolicies.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/gatewayclasses.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/grpcroutes.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/httproutes.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/listenersets.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/referencegrants.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/tcproutes.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/tlsroutes.gateway.networking.k8s.io serverside-applied
customresourcedefinition.apiextensions.k8s.io/udproutes.gateway.networking.k8s.io serverside-applied
validatingadmissionpolicy.admissionregistration.k8s.io/safe-upgrades.gateway.networking.k8s.io serverside-applied
validatingadmissionpolicybinding.admissionregistration.k8s.io/safe-upgrades.gateway.networking.k8s.io serverside-applied
helm upgrade --install agentgateway-crds oci://ghcr.io/agentgateway/charts/agentgateway-crds --version v1.7.0 \
	--namespace agentgateway-system --create-namespace --wait
Release "agentgateway-crds" does not exist. Installing it now.
Pulled: ghcr.io/agentgateway/charts/agentgateway-crds:v1.7.0
Digest: sha256:946b739c0fcda4d47a72d1589d184b6fd507772e733cbedf02789bafd0468064
NAME: agentgateway-crds
LAST DEPLOYED: Mon Nov 16 19:25:41 2026
NAMESPACE: agentgateway-system
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
NOTES:
Thank you for installing the agentgateway-crds chart.

Your release "agentgateway-crds" has been deployed in the "agentgateway-system" namespace.

This chart installs the Custom Resource Definitions (CRDs) required by agentgateway.

To check the installed CRDs:

  kubectl get crds | grep agentgateway.dev
helm upgrade --install agentgateway oci://ghcr.io/agentgateway/charts/agentgateway --version v1.7.0 \
	--namespace agentgateway-system --wait
Release "agentgateway" does not exist. Installing it now.
Pulled: ghcr.io/agentgateway/charts/agentgateway:v1.7.0
Digest: sha256:40340d639e35206317c934e9af3e29575cf7c5377b2940679732baac6e108c15
NAME: agentgateway
LAST DEPLOYED: Mon Nov 16 19:25:43 2026
NAMESPACE: agentgateway-system
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
NOTES:
Thank you for installing the agentgateway chart.

Your release "agentgateway" has been deployed in the "agentgateway-system" namespace.

To check the status of the deployment:

  helm status agentgateway --namespace agentgateway-system

To view the resources created by this chart:

  kubectl get all -n agentgateway-system

To learn how to access and use agentgateway, please visit the official documentation:

  https://agentgateway.dev/docs/kubernetes/latest/

To uninstall the agentgateway deployment:

  helm uninstall agentgateway --namespace agentgateway-system
kubectl -n agentgateway-system wait deployment --all --for=condition=Available --timeout=180s
deployment.apps/agentgateway condition met
kubectl wait gatewayclass/agentgateway --for=condition=Accepted --timeout=120s
gatewayclass.gateway.networking.k8s.io/agentgateway condition met
kubectl apply -f test/extmcp/deploy/gateway.yaml
namespace/agent-gate-test created
gateway.gateway.networking.k8s.io/gateway created
kubectl -n agent-gate-test wait gateway/gateway --for=condition=Programmed --timeout=180s
gateway.gateway.networking.k8s.io/gateway condition met
docker build --tag agent-gate/extmcp-stubs:dev test/extmcp
#0 building with "default" instance using docker driver

#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 338B done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/golang:1.27.4
#2 DONE 1.2s

#3 [internal] load .dockerignore
#3 transferring context: 61B done
#3 DONE 0.0s

#4 [build 1/6] FROM docker.io/library/golang:1.27.4@sha256:5e8afd493a7124a8586e134d9b532731161d4b9becf9dee5a5956b107426f93d
#4 resolve docker.io/library/golang:1.27.4@sha256:5e8afd493a7124a8586e134d9b532731161d4b9becf9dee5a5956b107426f93d 0.0s done
#4 DONE 0.0s

#5 [internal] load build context
#5 transferring context: 314B done
#5 DONE 0.0s

#6 [build 4/6] RUN go mod download
#6 CACHED

#7 [build 6/6] RUN CGO_ENABLED=0 go build -o /out/ ./cmd/...
#7 CACHED

#8 [build 2/6] WORKDIR /src
#8 CACHED

#9 [build 3/6] COPY go.mod go.sum ./
#9 CACHED

#10 [build 5/6] COPY cmd ./cmd
#10 CACHED

#11 [stage-1 1/1] COPY --from=build /out/stub-gate /out/stub-mcp /
#11 CACHED

#12 exporting to image
#12 exporting layers done
#12 exporting manifest sha256:3ac180fdbf3ea9e7ada05dac37cd767fa79b0605e9b88c91aa60d1bf2dd05ad1 done
#12 exporting config sha256:16a087fc2f5eb2198d22f63ff44c4b3a10e2221fc270651b57f427d04cf46f7f done
#12 exporting attestation manifest sha256:56094fbb5ab97d9d9f3964b4086a2403b516bf6f77b52d339b921f3d8f58e704 0.0s done
#12 exporting manifest list sha256:f93e4d20d46a095b8c5e9b45daf5cd842ad8bd9c491a53e5f2fa7248afa1a890 0.0s done
#12 naming to docker.io/agent-gate/extmcp-stubs:dev
#12 naming to docker.io/agent-gate/extmcp-stubs:dev done
#12 unpacking to docker.io/agent-gate/extmcp-stubs:dev done
#12 DONE 0.1s
kind load docker-image agent-gate/extmcp-stubs:dev --name agent-gate-extmcp
Image: "agent-gate/extmcp-stubs:dev" with ID "sha256:f93e4d20d46a095b8c5e9b45daf5cd842ad8bd9c491a53e5f2fa7248afa1a890" not yet present on node "agent-gate-extmcp-control-plane", loading...
kubectl apply -f test/extmcp/deploy/stubs.yaml -f test/extmcp/deploy/routing.yaml
deployment.apps/stub-mcp created
service/stub-mcp created
deployment.apps/stub-gate created
service/stub-gate created
agentgatewaybackend.agentgateway.dev/stub-mcp created
httproute.gateway.networking.k8s.io/mcp created
agentgatewaypolicy.agentgateway.dev/gate created
agentgatewaypolicy.agentgateway.dev/gate-timeout created
kubectl -n agent-gate-test wait deployment --all --for=condition=Available --timeout=180s
deployment.apps/gateway condition met
deployment.apps/stub-gate condition met
deployment.apps/stub-mcp condition met
kubectl -n agent-gate-test wait agentgatewaypolicy --all --timeout=60s \
	--for=jsonpath='{.status.ancestors[0].conditions[?(@.type=="Accepted")].status}'=True
agentgatewaypolicy.agentgateway.dev/gate condition met
agentgatewaypolicy.agentgateway.dev/gate-timeout condition met
cd test/extmcp && go test -tags e2e -count=1 -v ./... | tee e2e.log
=== RUN   TestE2E
=== RUN   TestE2E/Ungated_tool
=== RUN   TestE2E/First_call_to_a_gated_tool
    e2e_test.go:428: approval reference: 6a38852f5409db17
=== RUN   TestE2E/Replay_after_approval
=== RUN   TestE2E/Second_replay_of_a_used_approval
=== RUN   TestE2E/Replay_with_changed_arguments
=== RUN   TestE2E/Replay_by_a_different_caller
=== RUN   TestE2E/Call_with_a_valid_token
    e2e_test.go:492: params as the gate received them: {"name":"echo","arguments":{"message":"claims"}}
    e2e_test.go:493: headers as the gate received them: map[mcp-protocol-version:[2025-11-25]]
=== RUN   TestE2E/Call_with_a_token_that_fails_validation
    e2e_test.go:506: client error: calling "tools/call": sending "tools/call": Unauthorized
=== RUN   TestE2E/Call_with_no_token
    e2e_test.go:506: client error: calling "tools/call": sending "tools/call": Unauthorized
=== RUN   TestE2E/Result_of_a_passed_call
    e2e_test.go:529: result as the gate received it: {"content":[{"type":"text","text":"echo: result"}]}
=== RUN   TestE2E/Gate_unavailable
    e2e_test.go:551: JSON-RPC error: code=-32603 message="guardrail check failed"
--- PASS: TestE2E (1.68s)
    --- PASS: TestE2E/Ungated_tool (0.01s)
    --- PASS: TestE2E/First_call_to_a_gated_tool (0.00s)
    --- PASS: TestE2E/Replay_after_approval (0.01s)
    --- PASS: TestE2E/Second_replay_of_a_used_approval (0.01s)
    --- PASS: TestE2E/Replay_with_changed_arguments (0.01s)
    --- PASS: TestE2E/Replay_by_a_different_caller (0.01s)
    --- PASS: TestE2E/Call_with_a_valid_token (0.00s)
    --- PASS: TestE2E/Call_with_a_token_that_fails_validation (0.00s)
    --- PASS: TestE2E/Call_with_no_token (0.00s)
    --- PASS: TestE2E/Result_of_a_passed_call (0.00s)
    --- PASS: TestE2E/Gate_unavailable (1.62s)
PASS
ok  	github.com/vanlabs-dev/agent-gate/test/extmcp	1.895s
=== RUN   TestDenyApproveReplay
--- PASS: TestDenyApproveReplay (0.01s)
=== RUN   TestResponseIsRecorded
--- PASS: TestResponseIsRecorded (0.00s)
=== RUN   TestNoSubIsDeniedWithNoPendingEntry
--- PASS: TestNoSubIsDeniedWithNoPendingEntry (0.00s)
=== RUN   TestResetClearsState
--- PASS: TestResetClearsState (0.00s)
PASS
ok  	github.com/vanlabs-dev/agent-gate/test/extmcp/cmd/stub-gate	0.018s
=== RUN   TestEchoIsRecorded
--- PASS: TestEchoIsRecorded (0.00s)
PASS
ok  	github.com/vanlabs-dev/agent-gate/test/extmcp/cmd/stub-mcp	0.013s
  OK  make test

RESULT: PASS
```
