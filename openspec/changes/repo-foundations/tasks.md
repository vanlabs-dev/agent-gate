# Tasks

Tasks marked **Maintainer** change a GitHub setting. The agent writes the command, stops, and continues after the maintainer confirms.

## 1. Go targets and the rename

- [ ] 1.1 Add the root `versions.env` with the four tool versions (design decision 2) and include it in the `Makefile`. Verify: `go run <module>@<version>` prints the pinned version for each of the four tools
- [ ] 1.2 Rename the step `e2e` to `e2e-run`, make `e2e` the full cluster test, and add `GO_MODULES` and `test-go` (design decision 1). Verify: `make test-go` exits 0 in under 10 s with no cluster, and `make e2e` exits 0 and prints 11 scenarios as passed
- [ ] 1.3 Add `.golangci.yml` and `lint-go` (design decision 3). Verify: `make lint-go` exits 0 with no change to a Go source file, and an unused variable added by hand makes it exit non-zero
- [ ] 1.4 Add `vuln` (design decision 7). Verify: `make vuln` exits 0 and prints the result for `test/extmcp`
- [ ] 1.5 Change `make test` to `make e2e` where it means the cluster test: `README.md`, the Dependencies section of `docs/conventions.md`, the comment in `test/extmcp/versions.env` and the comment in `test/extmcp/e2e_test.go`. Verify: `grep -rn 'make test'` finds no line that means the cluster test, outside `openspec/changes/archive/` and `docs/greenfield/extmcp-spike/`

## 2. Python scaffold

- [ ] 2.1 Create `agents/azure-ops/` as a `uv` project on Python 3.14 with the `dev` group `ruff` and `pytest`, and add the ignore lines to `.gitignore` (design decision 4). Verify: `readlink -f agents/azure-ops/.venv/bin/python` is under `~/.local/share/uv/`, and `git status` shows no file from `.venv`
- [ ] 2.2 Add `src/azure_ops/__init__.py` and `tests/test_smoke.py`. Verify: `uv run --locked pytest` in `agents/azure-ops/` reports 1 passed
- [ ] 2.3 Add `lint-python` and `test-python` to the `Makefile`. Verify: both exit 0, an unused import added by hand makes `make lint-python` exit non-zero, and a changed `pyproject.toml` with an old `uv.lock` makes `make test-python` exit non-zero

## 3. Workflow lint, secret scan and the joined targets

- [ ] 3.1 Add `lint-actions` and `secrets` (design decisions 2 and 6). Verify: both exit 0, and `make secrets` reports the number of commits it scanned
- [ ] 3.2 Add `test`, `lint` and `check` (design decision 1). Verify: on a clean clone in `~/scratch/`, `make lint`, `make test` and `make check` each exit 0, `make test` takes under 30 s, and `kind get clusters` shows no new cluster
- [ ] 3.3 Add a "Commands" section to `AGENTS.md` with the targets of design decision 1 and the tools each needs. Add the same commands to `README.md`. Verify: each target in the list exists in the `Makefile`, and each exits 0

## 4. CI, pins and updates

- [ ] 4.1 Pin the three actions in `.github/workflows/extmcp.yml` by commit SHA with a version comment, and change its command to `make e2e`. Verify: each SHA is the commit of the named tag, read with `gh api repos/<owner>/<repo>/git/ref/tags/<tag>`
- [ ] 4.2 Add `.github/workflows/ci.yml` with the five jobs of design decision 5, each action pinned by SHA. Verify: `make lint-actions` exits 0, and `grep -rn 'uses:' .github/workflows` shows a 40-character SHA on each line
- [ ] 4.3 Add `.github/dependabot.yml` (design decision 8). Verify: after the merge in task 6.1, the Dependabot page of the repo lists three entries with no config error
- [ ] 4.4 Add `.github/pull_request_template.md` (design decision 10) and `.github/rulesets/main.json` (design decision 9). Verify: `jq . .github/rulesets/main.json` parses, the bypass list is empty, and the six check names equal the job names of design decision 5
- [ ] 4.5 Update the "CI and CD" and "Repo layout" sections of `docs/conventions.md`: the checks that exist now, the checks that start in Phase 2, the root `versions.env`, and `agents/azure-ops/` as an existing path. Verify: each check named as existing is a job in a workflow file
- [ ] 4.6 Open the pull request from `repo-foundations`. Verify: `gh pr checks` shows the six checks of design decision 5 as passed, with those exact names

## 5. Proof of a failed check

- [ ] 5.1 Create the branch `demo/failed-checks` with the three commits of design decision 11 and open a pull request. Verify: `gh pr checks` shows `go`, `python` and `secrets` as failed, and each log names the planted defect. Record the pull request number in `docs/greenfield/repo-foundations/08-verify.md`

## 6. Protection of main

- [ ] 6.1 **Maintainer:** merge the pull request of task 4.6. Verify: the six checks pass on `main`
- [ ] 6.2 **Maintainer:** apply the ruleset with the command in design decision 9. Verify: `gh api repos/vanlabs-dev/agent-gate/rulesets` lists one active ruleset, and its rules and check names equal the file
- [ ] 6.3 **Maintainer:** push one empty commit to `main` directly. Verify: GitHub rejects the push. Record the output in `08-verify.md`
- [ ] 6.4 Read the merge state of the demo pull request. Verify: `gh pr view <number> --json mergeStateStatus` prints `BLOCKED`. Then close the pull request and delete the branch `demo/failed-checks`
- [ ] 6.5 **Maintainer:** enable secret scanning with push protection, and Dependabot security updates. Verify: `gh api repos/vanlabs-dev/agent-gate --jq .security_and_analysis` shows `secret_scanning`, `secret_scanning_push_protection` and `dependabot_security_updates` as `enabled`

## 7. Close Phase 0

- [ ] 7.1 Update the Status section of `docs/vision.md` and the roadmap where it states the current phase: Phase 0 is complete, and Phase 1 is next. Verify: no doc says that Phase 0 is in progress
- [ ] 7.2 Open the second pull request with the verify output, the ship note, the archive and the docs of task 7.1. Verify: the six checks pass, and the maintainer merges it through the ruleset with no bypass
