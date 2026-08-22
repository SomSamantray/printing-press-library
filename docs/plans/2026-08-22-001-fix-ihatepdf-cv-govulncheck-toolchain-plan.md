---
artifact_contract: ce-unified-plan/v1
artifact_readiness: implementation-ready
execution: code
product_contract_source: ce-plan-bootstrap
type: fix
title: "fix(ihatepdf-cv): align module Go floor with the catalog Govulncheck toolchain"
created: 2026-08-22
plan_depth: deep
target_cli: library/productivity/ihatepdf-cv
---

# fix(ihatepdf-cv): align module Go floor with the catalog Govulncheck toolchain

## Product Contract

### Summary

Make PR #1785's `Govulncheck` gate load and scan `ihatepdf-cv` under the repository's pinned Go 1.26.6 runtime by lowering only the new module's declared Go floor from 1.26.7 to 1.26.6, recording the generated-tree correction, and then rerunning the honest vulnerability and repository gates. Do not merge the PR as part of this work.

### Problem Frame

PR #1785 (`feat/ihatepdf-cv`) adds `library/productivity/ihatepdf-cv` as a standalone Go module. The requested CI job is the `Govulncheck` check from run `32558281336`, job `96995810829`. The failure is deterministic and occurs before vulnerability analysis:

| Evidence | Finding |
|---|---|
| Root `.go-version` | The catalog pins CI to `1.26.6`. |
| `.github/workflows/govulncheck.yml` | `actions/setup-go@v6` reads `.go-version`, then the job runs with `GOTOOLCHAIN=local`; the selected module is scanned with `govulncheck ./...`. |
| PR commit `1e0246ba` | The newly added `library/productivity/ihatepdf-cv/go.mod` declares `go 1.26.7`. |
| Failed job log | Setup succeeds with `go1.26.6`, and govulncheck reports `go.mod requires go >= 1.26.7 (running go 1.26.6; GOTOOLCHAIN=local)` while loading packages. |
| Working tree | The pre-existing, uncommitted change changes that directive to `go 1.26.6`; it is strong evidence of the minimal remedy, but must be validated rather than accepted blindly. |
| Other catalog modules | Multiple recent modules use `go 1.26.6`; there is no repository evidence that this CLI requires a newer language/runtime floor. |

This is a module/runtime contract mismatch, not a reported CVE, a govulncheck installation failure, a missing API, or a local/browser architecture problem. The scan never reached reachable-vulnerability analysis. Lowering the module directive is safe only if package loading, tidy state, build, tests, vet, and the full govulncheck invocation all pass under Go 1.26.6.

### Requirements

- **R1.** `library/productivity/ihatepdf-cv/go.mod` must be loadable by the exact Go version selected by the catalog workflows (`1.26.6`) with `GOTOOLCHAIN=local`.
- **R2.** The fix must preserve the module path, required dependency versions, `go.sum`, source code, local/browser-only architecture, and published CLI behavior.
- **R3.** The exact CI command `govulncheck ./...` must complete successfully and report no reachable vulnerabilities; do not replace it with `-scan=module`, a skip, an allowlist, or automatic toolchain download.
- **R4.** Any direct correction to generated CLI output must be durable under future reprints through the per-CLI patch ledger convention.
- **R5.** Do not fabricate APIs or add remote behavior to address a dependency metadata failure. Publish only after the required gates are honestly green, and do not merge PR #1785 in this task.

### Scope Boundaries

In scope:

- The one-line Go directive correction in `library/productivity/ihatepdf-cv/go.mod`.
- A narrowly scoped `.printing-press-patches/` record explaining why the generated module floor must not exceed the catalog toolchain floor.
- Local validation and a CI rerun of the selected module's package loading, tests/build/vet, and govulncheck gate.

Out of scope:

- Changing the repository-wide `.go-version` from 1.26.6 to 1.26.7. That would widen the blast radius across every Go workflow and all standalone CLI modules without evidence that the catalog should move its pinned runtime.
- Changing `.github/workflows/govulncheck.yml`, installing a newer Go dynamically, setting `GOTOOLCHAIN=auto`, weakening scanner flags, or suppressing findings.
- Updating dependencies, `go.sum`, generated Go source, manifests, registry output, `cli-skills/`, README/SKILL behavior, or the local/browser-only product surface unless a validation command exposes a separate, independently evidenced issue. Such an issue is a follow-up, not permission to broaden this fix silently.
- Merging PR #1785 or treating a successful local command as a substitute for the GitHub required checks.

### Key Decisions

- **KD1 — Align the module floor downward, not the catalog runtime upward.** The workflow is intentionally pinned through the root `.go-version`, and the failure explicitly says the local runtime is too old for this module. The branch's one-line working-tree change is the smallest correction and matches the version used by other recent modules.
- **KD2 — Preserve the dependency graph.** The failure occurs while parsing the module's Go requirement, before dependency vulnerability reachability. Change only the `go` directive; do not use `go mod tidy` as a reason to opportunistically upgrade or reorder dependencies. If Go 1.26.6 identifies a dependency with an incompatible minimum, stop and report that separate fact instead of masking it.
- **KD3 — Keep the scanner strict and identical to CI.** A package-load pass is necessary but not sufficient. The implementation must run the same pinned govulncheck version and `./...` target, and must treat any actual reachable finding as a real remediation decision.
- **KD4 — Record the generated-output correction.** `ihatepdf-cv` is generated output and already carries `.printing-press-patches/.gitkeep`. Add one self-contained patch entry rather than leaving the rationale only in a commit message; this prevents a future reprint from recreating a module floor that the catalog cannot execute.

## Root-Cause Hypotheses and Evidence to Verify

1. **Confirmed primary cause: module Go floor exceeds CI runtime.** The failed job setup reports Go 1.26.6, while the committed module says `go 1.26.7`; the exact loader error names both versions and `GOTOOLCHAIN=local`. Verification is the pre-fix reproduction and the post-fix `go list ./...`/govulncheck run under Go 1.26.6.
2. **Rejected as the immediate cause: a vulnerable dependency.** No CVE or `GO-*` finding appears in the job. The scanner exits during package loading. After the directive correction, rerun the full scan before claiming the gate is fixed; a later vulnerability finding would be a separate blocker.
3. **Unlikely immediate cause: a stale or untidy dependency graph.** The logged error is a Go-version rejection, and govulncheck installation completed. Verify `go mod tidy -diff` after the directive change. If it reports changes, inspect them rather than committing an unrelated dependency rewrite.
4. **Possible secondary cause: code or dependency requires Go 1.26.7.** No source evidence currently establishes that requirement. `go list ./...`, `go build ./...`, `go vet ./...`, and `go test ./...` under Go 1.26.6 are the conservative proof. If any dependency's own `go.mod` requires a higher version, stop without changing the root pin or enabling toolchain downloads.
5. **Possible recurrence cause: generator environment drift.** The PR was generated with a 1.26.7 directive while the published catalog remains pinned at 1.26.6. The immediate PR fix belongs in the published artifact; a generator/template investigation may be opened separately if the same pattern reproduces. Do not mix a generator-repo change into this catalog PR without evidence and a local generator checkout.

## Implementation Units

### U1. Normalize `ihatepdf-cv`'s module floor

**Goal.** Make the module's declared minimum exactly match the Go runtime used by the catalog's CI workflows.

**File.** `library/productivity/ihatepdf-cv/go.mod`

**Changes.**

1. Finalize the existing working-tree edit so the directive is exactly `go 1.26.6` instead of `go 1.26.7`.
2. Confirm the module path remains `github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv`.
3. Leave every `require` version, indirect dependency comment, and `go.sum` entry unchanged unless a validation command proves a required, version-specific correction; do not proactively tidy or upgrade.
4. Do not change root `.go-version` or workflow configuration.

**Acceptance.** The only implementation diff in this unit is the one-line directive change, and Go 1.26.6 with `GOTOOLCHAIN=local` can load all packages in the module.

### U2. Preserve the correction in the generated-tree patch ledger

**Goal.** Make the module/runtime contract durable across future generated reprints.

**New file.** `library/productivity/ihatepdf-cv/.printing-press-patches/go-directive-matches-catalog-toolchain.json`

**Changes.** Add a schema-version-2 patch object with:

- `id`: `go-directive-matches-catalog-toolchain`, matching the patch filename;
- `applied_at`: the implementation date;
- `base_run_id`: `20260822-095304-45ca41a1` from `.printing-press.json`;
- `base_printing_press_version`: `4.31.1` from `.printing-press.json`;
- a concise `summary` stating that the module Go floor must not exceed the catalog's pinned Go toolchain;
- a `reason` citing the `GOTOOLCHAIN=local` package-load failure and the root `.go-version` contract;
- `files`: `["go.mod"]`;
- `validated_outcome`: the successful Go 1.26.6 package load and govulncheck result after implementation.

Do not edit `.printing-press-patches/.gitkeep` or create a legacy single-array patch file.

**Acceptance.** The JSON parses, names only `go.mod`, carries the manifest's run/version provenance, and describes the durable behavioral contract rather than a transient CI workaround.

### U3. Revalidate the exact gate and stop before merge

**Goal.** Prove the fix removes the loader failure without weakening security gates or product scope.

**Files.** No additional source files.

**Approach.** Run the validation sequence below from the CLI module with the catalog's Go 1.26.6 runtime and `GOTOOLCHAIN=local`. Then push the minimal fix, wait for GitHub to rerun PR #1785, and inspect the named Govulncheck job plus all required checks. Do not merge.

**Acceptance.** The exact job no longer fails at package loading, govulncheck completes with no reachable findings, and no unrelated file or generated mirror is changed.

## Test and Validation Scenarios

No new production tests are expected for a module-directive-only correction. The existing test suite must still be run during implementation; no tests are to be run during this planning stage.

1. **Static contract.** Assert root `.go-version` is `1.26.6`, `library/productivity/ihatepdf-cv/go.mod` declares `go 1.26.6`, and the module path is unchanged. Confirm `go.sum` is byte-for-byte unchanged from the pre-fix PR state.
2. **Exact package-load reproduction.** With Go 1.26.6 and `GOTOOLCHAIN=local`, run `go list ./...` from `library/productivity/ihatepdf-cv`; it must not emit the prior `requires go >= 1.26.7` error. This isolates the failure before invoking the security scanner.
3. **Tidy stability.** Run `go mod tidy -diff` from the module. It must report no diff. If it proposes dependency or sum changes, stop and review them; do not accept them as incidental cleanup.
4. **Build and static analysis.** Run `go build ./...` and `go vet ./...` under Go 1.26.6. Both must pass without automatic toolchain selection.
5. **Existing unit/integration tests.** Run `go test ./...` under Go 1.26.6. Existing tests must pass; no test additions are required for this metadata-only fix.
6. **Exact vulnerability gate.** Install the same scanner as CI (`go install golang.org/x/vuln/cmd/govulncheck@v1.3.0`) and run `govulncheck ./...` with `GOTOOLCHAIN=local`. It must reach analysis and exit zero with no reachable vulnerability findings. Do not substitute module/SBOM scanning or ignore flags.
7. **Artifact and scope review.** Validate the patch JSON, run `git diff --check`, inspect `git diff --stat` and `git status`, and confirm the implementation diff contains only `go.mod` plus the single patch record. Do not add `registry.json` or `cli-skills/pp-ihatepdf-cv/SKILL.md`; those are post-merge generated mirrors.
8. **Remote CI confirmation.** After the fix is committed to the PR branch, confirm the selected-module log shows Go 1.26.6, package loading succeeds, and the `Govulncheck` check is green. Also confirm the existing Verify, Verify SKILL.md, MCP manifest, supply-chain, and Greptile policy checks remain green. A local pass alone does not satisfy this scenario.

## Files to Modify

- `library/productivity/ihatepdf-cv/go.mod` - change only the module's `go` directive from 1.26.7 to the catalog-pinned 1.26.6 floor.

## New Files

- `library/productivity/ihatepdf-cv/.printing-press-patches/go-directive-matches-catalog-toolchain.json` - durable generated-tree patch record for the module/toolchain compatibility correction.

## Files Explicitly Not to Modify

- `.go-version` - repository-wide CI toolchain pin; retain `1.26.6`.
- `.github/workflows/govulncheck.yml` - the workflow is correctly exposing the mismatch and must remain strict.
- `library/productivity/ihatepdf-cv/go.sum` and all dependency/source files - no evidence requires changes.
- `registry.json` and `cli-skills/pp-ihatepdf-cv/SKILL.md` - generated post-merge artifacts.

## Dependencies and Ordering

1. U1 must be completed before U2's `validated_outcome` is finalized.
2. U1 and U2 must be complete before U3's local validation and diff review.
3. U3 local validation must pass before pushing the branch for remote CI confirmation.
4. Remote CI confirmation must pass before publication/merge decisions; this task ends with PR #1785 open and not merged.

## Risks / Open Questions

### Blocking risks

- If Go 1.26.6 reports that a direct or transitive dependency requires Go 1.26.7, the one-line fix is insufficient. Do not enable `GOTOOLCHAIN=auto`, raise `.go-version`, or pin dependencies speculatively. Capture the exact dependency and open a narrowly scoped follow-up decision.
- If govulncheck reaches analysis and reports an actual reachable vulnerability, the CI failure is no longer only a loader mismatch. Remediate or document that finding separately; never declare the gate fixed based only on package loading.

### Non-blocking follow-up

- Investigate the generator/publish path that emitted `go 1.26.7` while the catalog root remained at 1.26.6. That belongs in the generator repository or a separately evidenced catalog issue, not in this narrowly scoped PR repair.
- Consider a future consistency guard that rejects a newly published module whose `go` directive exceeds root `.go-version`; do not add that repository-wide guard to this PR.

## Definition of Done

- [ ] `library/productivity/ihatepdf-cv/go.mod` declares `go 1.26.6`; module path and dependency graph are unchanged.
- [ ] The generated-tree patch record is valid, provenance-bearing, and committed alongside the one-line correction.
- [ ] `go list ./...`, `go mod tidy -diff`, `go build ./...`, `go vet ./...`, and `go test ./...` pass with Go 1.26.6 and `GOTOOLCHAIN=local`.
- [ ] The exact `govulncheck@v1.3.0 ./...` invocation reaches package analysis and exits zero with no reachable findings.
- [ ] GitHub rerun of Govulncheck and all other required PR checks is green; no generated mirrors were hand-edited.
- [ ] No APIs, browser-only capabilities, or local-only boundaries were fabricated or widened.
- [ ] PR #1785 remains unmerged; publication proceeds only after the honest gates above are green.
