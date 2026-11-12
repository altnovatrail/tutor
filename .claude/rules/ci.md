---
paths:
  - ".github/**"
  - ".devcontainer/**"
  - "justfile"
  - ".sonarcloud.properties"
---

# CI, the release and the devcontainer

## CI

`.github/workflows/ci.yml` runs NovaTrail's shared checks from `.github`'s shared workflows, pinned to a release of `.github` by SHA with its version beside it (R03 .github): "Secrets" (gitleaks), "DCO", "Documents" — `docs-check-repo` — and "Go", the shared `go.yml`, whose recipes are the go rule, [go.md](go.md) (R13). Beside them run tutor's own jobs, in the `toolchain` stage: "Samples", `just ci-sample-check` (R69), and "Canvas", `just ci-canvas-check` (R76), the eval rule's checks, [eval.md](eval.md). CodeQL, Scorecard and SonarCloud run beside them too, and every pull request runs every check, documents only or not; a merge waits for them all and for `codecov/patch` (R30 spec). Their findings are closed before the next task, by rule 12 of RUN.md. A key-like constant in a test fails "Secrets" over the branch's whole history: write test keys as plain words.

`.github/workflows/eval.yml` runs the eval with the platform's keys for eval from Secret Manager of `infra`, in the GitHub environment `eval`, and only by hand until the keys are there (R90; Д6 of DEBT.md).

## The release

A release of tutor comes out by `.github`'s shared `release.yml` after each green merge to `main` that changes what the module is built from: the tag `vX.Y.Z` of the Go module, which api builds from, and its files, which `just release-files` builds from the tree `git archive` writes — the source, the open views of the measurement's tasks and `SHA256SUMS` (R62, R64). `paths` in `release.yml` lists what the module is built from, the files the code embeds with `//go:embed` among them, and `just release-files-test` holds that list to the code. Nobody tags by hand.

## The devcontainer

The container is spec's, as `just new-repo-draft tutor go 3.6 <tag>` builds it from a copy of spec in the scratchpad, never inside the submodule: a move to a tag of spec takes its `.devcontainer/` from there and adds tutor's own differences again — golangci-lint, under GPL-3.0 as an exception named by R12 and run by `just lint` (R13), and the image of the submodule's contracts that `buf` builds for spec's `sample-check` and `report-check` (R69). Every tool is pinned by exact version, and every archive by its sha256 beside it for each architecture (RULES.md spec, Hard rules). A raise of Go of tutor's own changes, at once, `GO_VERSION` and both `GO_SHA256_*` of the `toolchain` stage, with the sums of go.dev's list checked against the downloaded archives, and the `toolchain` line of `go.mod` (the 2026-11-11 amendment to R12). The `go` line of `.devcontainer/gopls/go.mod` is spec's: as its header says, a repository takes it with its next move to a tag (the 2026-11-12 amendment to R12). A rebuild of the container is the author's step.
