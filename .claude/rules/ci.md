---
paths:
  - ".github/**"
  - ".devcontainer/**"
  - "justfile"
  - ".sonarcloud.properties"
---

# CI, the release and the devcontainer

## CI

`.github/workflows/ci.yml` runs NovaTrail's shared checks from `.github`'s shared workflows, pinned to a release of `.github` by SHA with its version beside it (R03 .github), every `uses:` of `.github/workflows/` at the same one, which Dependabot raises in one pull request, the group `shared-workflows` (Д88 spec; R117): "Secrets" (gitleaks), "DCO", "Documents" — `docs-check-repo` — and "Go", the shared `go.yml`, whose recipes are the go rule, [go.md](go.md) (R13). Beside them run tutor's own jobs, in the `toolchain` stage: "Samples", `just ci-sample-check` (R69), and "Canvas", `just ci-canvas-check` (R76), the eval rule's checks, [eval.md](eval.md). The jobs in the stage take its image from the cache of GitHub Actions, by the key of the runner and the sha256 of `.devcontainer/Dockerfile`, and build it only on a miss (the 2026-11-09 amendment to R31 spec; Д146 spec; R117): `go.yml`'s jobs by their own steps and the file `toolchain.tar`, "Samples" and "Canvas" by a copy of those steps, and "Documents" by `spec`'s `toolchain.sh` and the file `novatrail-toolchain.tar`, an entry of its own under the same key. A move to a new release of `.github` compares the copy with that release's `go.yml`: a copy that drifts costs a second build and entry, not a verdict. A change of the Dockerfile is a miss, so its pull request builds the image from the network. The release always builds it, and so does "CodeQL / analyze (go)" at `v0.1.24`, for the Go of the stage; one file of the image for all jobs and CodeQL's Go without the image come with the release of `.github` of row 7.13 of spec's COORDINATION.md (Д16 of run/DEBT.md). CodeQL, Scorecard and SonarCloud run beside them too, and every pull request runs every check, documents only or not; a merge waits for them all and for `codecov/patch` (R30 spec). Their findings are closed before the next task, by rule 12 of RUN.md. A key-like constant in a test fails "Secrets" over the branch's whole history: write test keys as plain words.

`.github/workflows/eval.yml` runs the eval with the platform's keys for eval from Secret Manager of `infra`, in the GitHub environment `eval`, and only by hand until the keys are there (R90; Д6 of run/DEBT.md).

## The release

A release of tutor comes out by `.github`'s shared `release.yml` after each green merge to `main` that changes what the module is built from: the tag `vX.Y.Z` of the Go module, which api builds from, and its files, which `just release-files` builds from the tree `git archive` writes — the source, the open views of the measurement's tasks and `SHA256SUMS` (R62, R64). `paths` in `release.yml` lists what the module is built from, the files the code embeds with `//go:embed` among them, and `just release-files-test` holds that list to the code. Nobody tags by hand.

## The devcontainer

The container is spec's, as `just new-repo-draft tutor go 3.6 <tag>` builds it from a copy of spec in the scratchpad, never inside the submodule: a move to a tag of spec takes its `.devcontainer/` from there and adds tutor's own differences again — golangci-lint, under GPL-3.0 as an exception named by R12 and run by `just lint` (R13), and the image of the submodule's contracts that `buf` builds for spec's `sample-check` and `report-check` (R69). Every tool is pinned by exact version, and every archive by its sha256 beside it for each architecture (RULES.md spec, Hard rules). A raise of Go of tutor's own changes, at once, `GO_VERSION` and both `GO_SHA256_*` of the `toolchain` stage, with the sums of go.dev's list checked against the downloaded archives, and the `toolchain` line of `go.mod` (the 2026-11-11 amendment to R12). The `go` line of `.devcontainer/gopls/go.mod` is spec's: as its header says, a repository takes it with its next move to a tag (the 2026-11-12 amendment to R12). A rebuild of the container is the author's step.
