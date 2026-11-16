# agent-gate

agent-gate is an open-source approval and policy gate for AI agent tool calls. It decides each MCP tool call against policy, holds the risky ones for a human, and keeps an audit record that anyone can verify.

It is not a gateway. It plugs into [agentgateway](https://github.com/agentgateway/agentgateway) as an extension.

## Status

Phase 0 of the [roadmap](docs/roadmap.md). A spike tests the integration with agentgateway on `kind`. No gate exists yet.

## Documents

- [Vision](docs/vision.md): what the project is, and what is out of scope
- [Architecture options](docs/architecture/options.md)
- [Decision records](docs/adr/)
- [Roadmap](docs/roadmap.md)
- [Conventions](docs/conventions.md)

## Licence

[Apache-2.0](LICENSE)
