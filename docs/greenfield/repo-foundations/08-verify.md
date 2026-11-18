# Verify: repo-foundations

Session 4, 2026-11-18.

## Task evidence

| Task | Result |
|---|---|
| 1.2 | `make e2e` exit 0, 11 scenarios passed. `make test-go` 0.4 s |
| 1.3 | `make lint-go` 0 issues. An unused variable and a format error each give a non-zero exit |
| 2.1 | `.venv/bin/python` resolves to `~/.local/share/uv/python/cpython-3.14.10-linux-x86_64-gnu/bin/python3.14` |
| 2.3 | An unused import fails `make lint-python`. A stale `uv.lock` fails `make test-python` |
| 3.1 | `make secrets`: 31 commits scanned, no leaks. A fake key on another local branch: `make secrets` exit 0, a scan of that branch alone exit 1, the default all-refs scan exit 1 (review finding F1 reproduced). A key in a `.txt` file was not found, so the demo key is in a `.py` file |
| 3.2 | Clean clone in `~/scratch/2026-11-18-repo-foundations-clean-clone/`: `make lint`, `make test` (0.6 s) and `make check` exit 0. `kind get clusters`: none |
| 4.3 | Three `Dependabot` check runs on `main` (`ccc1b02`), each `success`. Dependabot opened PR #4 (`uv_build` range) |
| 4.6 | PR #3: six checks passed. The `python` log: `Found version for uv in agents/azure-ops/pyproject.toml: 0.12.5` |
| 5.1 | PR #3 squash-merged as `ccc1b02`. The six checks passed on `main` |
| 6.1 | PR #5 from `demo/failed-checks`: `go`, `python`, `secrets` failed; `actionlint`, `govulncheck`, `agentgateway compatibility test` passed |

Failed logs of PR #5 (run 44240686909):

```text
go: test/extmcp/cmd/stub-gate/demo_lint.go:4:6: func demoUnused is unused (unused)
python: PLR0133 Two constants compared in a comparison, consider replacing `1 == 2`
python: FAILED tests/test_demo_failure.py::test_planted_failure - AssertionError: planted failure for the repo-foundations demo
secrets: WRN leaks found: 1
```

The `secrets` log gives a count only. With `--verbose`, the same scan prints `RuleID: generic-api-key` and `File: demo/config.py`, with the value redacted. `make secrets` now runs with `--verbose` (decided by the maintainer, 2026-11-18).

| Task | Result |
|---|---|
| 7.1 | Ruleset `27522673` active. Its rules, check names, approvals, strict mode and bypass list equal `.github/rulesets/main.json`. GitHub adds `require_extra_approval_for_unattributed_changes: true` by default |
| 7.2 | Direct push of an empty commit to `main` rejected (output below). The local commit was reset |
| 7.3 | PR #5 `mergeStateStatus`: `BLOCKED`. Closed, not merged. Branch `demo/failed-checks` deleted |

```text
remote: error: GH013: Repository rule violations found for refs/heads/main.
remote: - Changes must be made through a pull request.
remote: - 6 of 6 required status checks are expected.
 ! [remote rejected] main -> main (push declined due to repository rule violations)
```
