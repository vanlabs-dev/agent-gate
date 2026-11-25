# Verify: gate-core

- Date: 2026-11-25
- Command: `bash ~/.claude/skills/greenfield/scripts/verify.sh gate-core .` from the repo root, on `main` at `f14d24d` plus task 9.4 checked
- Exit: 0, `RESULT: PASS`

The script runs `make test` only. Beyond it, on the same code:

- `make check`, `make vuln` and `make secrets` pass locally.
- `make e2e-gate` and `make e2e` pass from scratch locally, and as the required checks `gate end-to-end test` and `agentgateway compatibility test` on PR #10.
- `make e2e-gate` with `E2E_TAGS=none` exits non-zero.

## Output

```text
=== greenfield verify: gate-core ===
Project root: /home/van/src/github/vanlabs-dev/agent-gate
Artifact dir: /home/van/src/github/vanlabs-dev/agent-gate/docs/greenfield/gate-core

Checking greenfield artifacts...
  OK  01-brainstorm.md
  OK  07-review.md
  note  09-ship.md not present yet

Checking OpenSpec change...
  OK  openspec/changes/gate-core

Running project checks (if present)...
go test test/extmcp
ok  	github.com/vanlabs-dev/agent-gate/test/extmcp/cmd/stub-gate	0.007s
ok  	github.com/vanlabs-dev/agent-gate/test/extmcp/cmd/stub-mcp	0.005s
go test gate
ok  	github.com/vanlabs-dev/agent-gate/gate/cmd/agent-gate	0.030s
ok  	github.com/vanlabs-dev/agent-gate/gate/cmd/agent-gate-audit	0.008s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/approval	0.012s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/audit	0.005s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/canonical	0.002s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/extmcp	0.010s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/identity	0.028s
ok  	github.com/vanlabs-dev/agent-gate/gate/internal/policy	0.008s
?   	github.com/vanlabs-dev/agent-gate/gate/internal/postgres	[no test files]
cd agents/azure-ops && uv run --locked pytest
============================= test session starts ==============================
platform linux -- Python 3.14.10, pytest-9.2.0, pluggy-1.7.0
rootdir: /home/van/src/github/vanlabs-dev/agent-gate/agents/azure-ops
configfile: pyproject.toml
collected 1 item

tests/test_smoke.py .                                                    [100%]

============================== 1 passed in 0.01s ===============================
  OK  make test

RESULT: PASS
```
