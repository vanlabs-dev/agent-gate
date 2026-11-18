# Review: repo-foundations

- Date: 2026-11-18
- Session: 3, outside voice. Inputs: the [brainstorm](01-brainstorm.md), the [OpenSpec change](../../../openspec/changes/repo-foundations/) and the project docs. No chat history.
- Checks: the four pinned tools were run with `go run <module>@<version>` on Go 1.27.4. The repo settings were read with `gh api`. The greenfield verify script and the `astral-sh/setup-uv` README were read. All on 2026-11-18.

## Verdict

The plan is sound and the scope fits Phase 0. Nothing to cut. F1 and F2 break the migration plan as written: fix both before apply. F3 to F7 each make one task fail or make CI differ from the machine. The rest is wording and small gaps.

## Design claims checked

Confirmed:

- The lint set of design decision 3 reports 0 issues on `test/extmcp` in three runs, and the format check is clean. No Go source file must change.
- `golangci-lint`, `govulncheck` and `actionlint` build with `go run` and print their pinned version.
- The repo is public, has no ruleset, and allows merge commits, squash and rebase.
- `openspec validate repo-foundations --strict` passes. The `MODIFIED` delta holds the full requirement with both scenarios.
- The root holds no `pyproject.toml`, so the verify script runs `make test` and no bare `pytest`.
- The ship note of `extmcp-spike` confirms ADR 0002 and ADR 0009. The Phase 0 exit test is met when this change ships.

Not checked: the ruleset JSON against the API (the file does not exist yet), the Dependabot `uv` entry with a group, the content of the `ubuntu-24.04` runner image, push protection at the user account level.

## Findings

| ID | Level | Finding | Proposed change |
|---|---|---|---|
| F1 | High | `gitleaks git` scans all refs by default (`git log --all`, `sources/git.go` in v8.30.1). `actions/checkout` with `fetch-depth: 0` fetches all branches. While `demo/failed-checks` is on the remote, `secrets` fails on each pull request and on `main`. Task 6.1 then fails, and the demo proves nothing about its own pull request. Later, one branch with a caught secret blocks each merge, with no bypass. | Run `gitleaks git --redact --log-opts="--full-history HEAD"` in `make secrets`. On a pull request, `HEAD` is the merge commit, so the scan covers `main` and the commits of that pull request. Checked: 29 commits scanned, exit 0. |
| F2 | High | Design decision 11 says the fake credential is pushed before push protection is on. Secret scanning and push protection are on now (`security_and_analysis`, read 2026-11-18). A fake credential in a provider format is rejected at the push. | Plant a generic key with no provider format, for example `api_key = "<32 random characters>"`. The `gitleaks` rule `generic-api-key` reports it (checked, exit 1). Push protection checks provider patterns, and non-provider patterns are off. If the push is still rejected, use the bypass link with the reason "used in tests". In task 6.5 and the proposal, only Dependabot security updates are left to enable. Keep the three-value verify. |
| F3 | Medium | The demo branch starts from `repo-foundations` before the merge. After a squash or rebase merge, a file that both sides add in different forms conflicts. `mergeStateStatus` is then `DIRTY`, not `BLOCKED`, and task 6.4 fails. The demo diff also shows the full change, not three defects. | Create `demo/failed-checks` from `main` after the merge (migration step 3) and before the ruleset (step 4). Its diff is the three defects only. Reorder tasks 5.1 and 6.1. |
| F4 | Medium | Design decision 4 says `astral-sh/setup-uv` reads `required-version` from `pyproject.toml`. The action reads the repo root only. The file is in `agents/azure-ops/`, so the action installs the latest `uv`. CI then differs from the machine, or fails on a version mismatch. | Set `working-directory: agents/azure-ops` on the action. Verify in task 4.6: the job log shows the `uv` version of `required-version`. |
| F5 | Medium | `actionlint` runs `shellcheck` and `pyflakes` when they are on `PATH`. The machine has neither. The runner image is expected to have `shellcheck`. The probe result "no findings" was made without it. CI can report a finding that `make lint-actions` cannot show, against the first goal of the design. | Run `actionlint -shellcheck= -pyflakes=` in `make lint-actions`. Both places then run the same checks. The workflow scripts are short. Alternative: install `shellcheck` in `~/.local/bin`, a new tool binary. |
| F6 | Medium | Task 1.1 cannot pass. `gitleaks` built with `go run` prints `version is set by build process`, not `v8.30.1`. | Verify three tools by their version output. For `gitleaks`, verify that the `Makefile` line reads `GITLEAKS_VERSION` and that `make secrets` exits 0. |
| F7 | Medium | Task 1.5 cannot pass and misses one file. `docs/adr/0002-integrate-through-agentgateway-extmcp.md` line 28 names `make test` as the compatibility test. The main spec changes only at the archive (task 7.2). The change directory and `docs/greenfield/repo-foundations/` name `make test` in both meanings. | Add ADR 0002 to task 1.5 and to the proposal. A command name is not the decision, so no new ADR. Exclude `openspec/` and `docs/greenfield/` from the grep. Check the main spec in task 7.2. |
| F8 | Low | The `src/` layout needs an installed project, or the smoke test cannot import `azure_ops`. The design names no build backend, and the proposal lists no such dependency. | Create the project with `uv init --package`. Add the build backend `uv_build` to the dependency list of the proposal. |
| F9 | Low | `--python-preference only-managed` holds for the first virtual environment only. `uv run --locked` makes a new one in a clean clone and in CI with the default preference. | Set `python-preference = "only-managed"` under `[tool.uv]` in `pyproject.toml`. |
| F10 | Low | The scenarios "Lint error" and "Failed test" say "the lint check of that language" and "the test check of that language". The design has one check for each language. Success signal 3 of the brainstorm lists eight checks. | Reword both scenarios: "the check of that language fails". |
| F11 | Low | The requirement "Dependency updates are proposed automatically" has no proof in this change. Task 4.3 reads the config only. Dependabot runs each entry once when the config reaches `main`. | Extend task 4.3: after task 6.1, the Dependabot log shows one finished run for each of the three entries. |
| F12 | Low | Task 7.1 misses the status badge in `README.md` (`status-phase 0`) and the roadmap checkpoint "End of Phase 0", which the spike has decided. | Add both to task 7.1. |
| F13 | Low | The design gives the `POST` command only. A second `POST` makes a second ruleset. Phase 2 adds checks, so the file changes again. | Add the update command to the design and to `docs/conventions.md`: `gh api --method PUT repos/vanlabs-dev/agent-gate/rulesets/<id> --input .github/rulesets/main.json`. |
| F14 | Low | Task 6.3 pushes an empty commit to `main`. If the ruleset is wrong, the commit lands and only a force push removes it. | State the failure case in the task: if the push passes, stop, leave the commit, and correct the ruleset. Use a subject that can stay in the history. |
| F15 | Low | CI uses the exact Go version of `go.mod`, and Dependabot does not update it. The stubs reach `net/http`. Each Go security release can fail `govulncheck` on each pull request until a manual update merges. The design accepts the advisory risk and does not state this cost. | Accept. Add one line to the risk list: a Go security release needs a manual update of `go.mod`. |
| F16 | Low | The brainstorm says the verify script proves lint and unit tests. The script runs `make test` only. | In Session 4, add the output of `make check` to `08-verify.md` below the script output. |

## Checked and fine

- No bypass with one maintainer: the recovery path is an edit of the ruleset, and GitHub records it.
- `go run` flattens each tool exit code to 1. `make` needs only zero or non-zero.
- A required check with a combined lint and test step still fails when one step fails.
- Over-engineering: nothing to cut. The pull request template is four lines. The ruleset file is one file.

## Resolution

Decided by the maintainer on 2026-11-18.

- F1 to F14 and F16: the proposed change is folded into the OpenSpec change through `/opsx:update`.
- F1: the spec also gets the scenario "Secret on another branch", and task 3.1 tests it.
- F5: `actionlint` runs with `-shellcheck= -pyflakes=`. No `shellcheck` is installed.
- F15: accepted. The risk list of the design has one new line.

The task numbers in the findings are the numbers before the update. After it, the merge is task 5.1, the demo is task 6.1, the protection tasks are 7.1 to 7.4, and the close tasks are 8.1 and 8.2.

The change still has 25 tasks and passes `openspec validate repo-foundations --strict`.
