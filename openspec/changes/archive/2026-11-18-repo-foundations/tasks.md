# Tasks

Tasks marked **Maintainer** change a GitHub setting. The agent writes the command, stops, and continues after the maintainer confirms.

## 1. Go targets and the rename

- [x] 1.1 Add the root `versions.env` with the four tool versions (design decision 2) and include it in the `Makefile`. Verify: `go run <module>@<version>` prints the pinned version for `golangci-lint`, `govulncheck` and `actionlint`. `gitleaks` prints no version under `go run`: verify that its `Makefile` line reads `GITLEAKS_VERSION`
- [x] 1.2 Rename the step `e2e` to `e2e-run`, make `e2e` the full cluster test, and add `GO_MODULES` and `test-go` (design decision 1). Verify: `make test-go` exits 0 in under 10 s with no cluster, and `make e2e` exits 0 and prints 11 scenarios as passed
- [x] 1.3 Add `.golangci.yml` and `lint-go` (design decision 3). Verify: `make lint-go` exits 0 with no change to a Go source file, and an unused variable added by hand makes it exit non-zero
- [x] 1.4 Add `vuln` (design decision 7). Verify: `make vuln` exits 0 and prints the result for `test/extmcp`
- [x] 1.5 Change `make test` to `make e2e` where it means the cluster test: `README.md`, the Dependencies section of `docs/conventions.md`, line 28 of `docs/adr/0002-integrate-through-agentgateway-extmcp.md` (the command name only), the comment in `test/extmcp/versions.env` and the comment in `test/extmcp/e2e_test.go`. Verify: `grep -rn 'make test' --exclude-dir=.git --exclude-dir=openspec --exclude-dir=greenfield .` finds no line that means the cluster test. The main spec under `openspec/specs/` changes at the archive (task 8.2)

## 2. Python scaffold

- [x] 2.1 Create `agents/azure-ops/` with `uv init --package` on Python 3.14, with the build backend `uv_build`, the `dev` group `ruff` and `pytest`, and `python-preference = "only-managed"` under `[tool.uv]`. Add the ignore lines to `.gitignore` (design decision 4). Verify: after `.venv` is deleted and `uv run --locked python -V` makes it again, `readlink -f agents/azure-ops/.venv/bin/python` is under `~/.local/share/uv/`, and `git status` shows no file from `.venv`
- [x] 2.2 Add `src/azure_ops/__init__.py` and `tests/test_smoke.py`. Verify: `uv run --locked pytest` in `agents/azure-ops/` reports 1 passed
- [x] 2.3 Add `lint-python` and `test-python` to the `Makefile`. Verify: both exit 0, an unused import added by hand makes `make lint-python` exit non-zero, and a changed `pyproject.toml` with an old `uv.lock` makes `make test-python` exit non-zero

## 3. Workflow lint, secret scan and the joined targets

- [x] 3.1 Add `lint-actions` with `-shellcheck= -pyflakes=` and `secrets` with `--log-opts="--full-history HEAD"` (design decisions 2 and 6). Verify: both exit 0, and `make secrets` reports the number of commits it scanned. Then commit a fake key (design decision 11) on a local branch that is not checked out: `make secrets` still exits 0. Delete that branch
- [x] 3.2 Add `test`, `lint` and `check` (design decision 1). Verify: on a clean clone in `~/scratch/`, `make lint`, `make test` and `make check` each exit 0, `make test` takes under 30 s, and `kind get clusters` shows no new cluster
- [x] 3.3 Add a "Commands" section to `AGENTS.md` with the targets of design decision 1 and the tools each needs. Add the same commands to `README.md`. Verify: each target in the list exists in the `Makefile`, and each exits 0

## 4. CI, pins and updates

- [x] 4.1 Pin the three actions in `.github/workflows/extmcp.yml` by commit SHA with a version comment, and change its command to `make e2e`. Verify: each SHA is the commit of the named tag, read with `gh api repos/<owner>/<repo>/git/ref/tags/<tag>`
- [x] 4.2 Add `.github/workflows/ci.yml` with the five jobs of design decision 5, each action pinned by SHA. `astral-sh/setup-uv` gets `working-directory: agents/azure-ops` (design decision 4). Verify: `make lint-actions` exits 0, and `grep -rn 'uses:' .github/workflows` shows a 40-character SHA on each line
- [x] 4.3 Add `.github/dependabot.yml` (design decision 8). Verify: after the merge in task 5.1, the Dependabot page of the repo lists three entries with no config error, and its log shows one finished run for each entry
- [x] 4.4 Add `.github/pull_request_template.md` (design decision 10) and `.github/rulesets/main.json` (design decision 9). Verify: `jq . .github/rulesets/main.json` parses, the bypass list is empty, and the six check names equal the job names of design decision 5
- [x] 4.5 Update the "CI and CD" and "Repo layout" sections of `docs/conventions.md`: the checks that exist now, the checks that start in Phase 2, the root `versions.env`, `agents/azure-ops/` as an existing path, and the command that updates the ruleset from its file (design decision 9). Verify: each check named as existing is a job in a workflow file
- [x] 4.6 Open the pull request from `repo-foundations`. Verify: `gh pr checks` shows the six checks of design decision 5 as passed, with those exact names, and the log of the job `python` shows the `uv` version that `required-version` names

## 5. Merge

- [x] 5.1 **Maintainer:** merge the pull request of task 4.6. Verify: the six checks pass on `main`

## 6. Proof of a failed check

- [x] 6.1 Create the branch `demo/failed-checks` from `main` with the three commits of design decision 11 and open a pull request. Verify: `gh pr checks` shows `go`, `python` and `secrets` as failed and the other three checks as passed, and each failed log names the planted defect. Record the pull request number in `docs/greenfield/repo-foundations/08-verify.md`

## 7. Protection of main

- [x] 7.1 **Maintainer:** apply the ruleset with the `POST` command in design decision 9. Verify: `gh api repos/vanlabs-dev/agent-gate/rulesets` lists one active ruleset, and its rules and check names equal the file
- [x] 7.2 **Maintainer:** push one empty commit to `main` directly, with a subject that can stay in the history. Verify: GitHub rejects the push. Record the output in `08-verify.md`. If the push passes, stop: leave the commit, do not force push, and correct the ruleset
- [x] 7.3 Read the merge state of the demo pull request. Verify: `gh pr view <number> --json mergeStateStatus` prints `BLOCKED`. Then close the pull request and delete the branch `demo/failed-checks`
- [x] 7.4 **Maintainer:** enable Dependabot security updates. Secret scanning and push protection are on already. Verify: `gh api repos/vanlabs-dev/agent-gate --jq .security_and_analysis` shows `secret_scanning`, `secret_scanning_push_protection` and `dependabot_security_updates` as `enabled`

## 8. Close Phase 0

- [x] 8.1 Update the Status section of `docs/vision.md`, the status badge in `README.md`, and the roadmap where it states the current phase: Phase 0 is complete, and Phase 1 is next. Remove the roadmap checkpoint "End of Phase 0", which the spike has decided. Verify: no doc says that Phase 0 is in progress
- [x] 8.2 Open the second pull request with the verify output, the ship note, the archive and the docs of task 8.1. `08-verify.md` holds the output of the verify script and, below it, the output of `make check`. Verify: the six checks pass, the main spec `openspec/specs/agentgateway-integration/spec.md` names `make e2e`, and the maintainer merges the pull request through the ruleset with no bypass. Checked off at the archive, which is part of this pull request: the merge through the ruleset is its last step
