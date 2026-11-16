# agent-gate

agent-gate is an open-source approval and policy gate for AI agent tool calls. It decides each MCP tool call against policy, holds the risky ones for a human, and keeps an audit record that anyone can verify.

It is not a gateway. It plugs into [agentgateway](https://github.com/agentgateway/agentgateway) as an extension.

## Status

Phase 0 of the [roadmap](docs/roadmap.md). A compatibility test proves the integration with agentgateway on `kind`, with a stub in place of the gate. No gate exists yet.

## Run the compatibility test

The test proves the contract between agentgateway and a gate behind its `ExtMcp` extension. It creates a `kind` cluster, deploys a pinned agentgateway with a stub MCP server and a stub gate, and runs each scenario of the contract. Versions are in [`test/extmcp/versions.env`](test/extmcp/versions.env).

Required tools: Go, Docker, `kind`, `kubectl`, Helm 4 and `make`.

```sh
make test
```

The command exits 0 only when every scenario passes. It takes about one minute. It deletes and recreates the cluster `agent-gate-extmcp` on each run, and keeps its kubeconfig in `test/extmcp/.kube/`.

```sh
make clean
```

This deletes the cluster.

## Documents

- [Vision](docs/vision.md): what the project is, and what is out of scope
- [Architecture options](docs/architecture/options.md)
- [Decision records](docs/adr/)
- [Roadmap](docs/roadmap.md)
- [Conventions](docs/conventions.md)

## Licence

[Apache-2.0](LICENSE)
