# tutor

[![CI](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml)
[![Release](https://github.com/altnovatrail/tutor/actions/workflows/release.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/release.yml)
[![CodeQL](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml)
[![Latest release](https://img.shields.io/github/v/release/altnovatrail/tutor)](https://github.com/altnovatrail/tutor/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/altnovatrail/tutor)](go.mod)
[![Coverage](https://codecov.io/gh/altnovatrail/tutor/branch/main/graph/badge.svg)](https://codecov.io/gh/altnovatrail/tutor)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/16171/badge)](https://www.bestpractices.dev/projects/16171)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/altnovatrail/tutor/badge)](https://scorecard.dev/viewer/?uri=github.com/altnovatrail/tutor)
[![License](https://img.shields.io/github/license/altnovatrail/tutor)](LICENSE)

[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=code_smells)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)

The AI side of the NovaTrail tutor: it reads the learner's work, handwriting or text, into an observation, reviews the working for the first mistake, assesses an essay against the module author's rubric, and words the hint for the rung of the hint ladder that the platform's program chose. It holds the platform's prompts, one per AI capability and each with its own eval set, the adapters to AI providers, the exact models admitted for each capability, and the one function that admits an AI route for the learner's age category before every request; it reports each call's usage, by which api counts the platform AI quotas, and its eval runner is a CLI on the same code. It is a library inside api: it decides no verdict, picks no lesson or task and stores nothing, and its requests carry no identifier of the learner or the account: on the own-key route only the key itself goes along, to its own provider, which ties each call to the key owner's account.

A component of [NovaTrail](https://github.com/altnovatrail/spec), a universal open platform for adaptive learning, interactive lessons and AI tutoring. It belongs to stage 2, First slice with learners, with canvas, api, app, skill-map and infra; together with canvas it brings the stage's first code, which for tutor is the check board.

## Releases

A release comes out by itself after each merge into `main` that changes what the module is built from: its Go code, the files that code embeds, `go.mod`, `go.sum`, the `justfile`, the license, the Dockerfile, the release workflow or the file of open views below (R62 in [docs/decisions.md](docs/decisions.md)). Its number is the next patch of line 0.1, from v0.1.0. A release is the tag of the Go module, which api builds from, `go get github.com/altnovatrail/tutor@v0.1.0`, and its files: the source of that commit, `tutor_<version>_source.tar.gz`; from the first release after 2026-11-02, the open views of the measurement's tasks, `tutor_<version>_open-tasks.json`; and `SHA256SUMS` over them. The open views are what a learner sees of each of the 56 tasks of the measurements — its statement, its canvas and its answer slots, in spec's module format 0.2 up to v0.1.1 and 0.3 from the first release after task T30 of [RUN.md](RUN.md) — and none of its key, solution, traps or checks: until the modules repository has content, canvas's CI puts them into the build of its sample page, by the release's tag and the file's sum (R64). The file is `eval/open-tasks.json`, and `just open-tasks` writes it anew from the banks. Each file comes with its Sigstore bundle, `<file>.sigstore.json`. The releases are immutable, and `gh release verify <tag> -R altnovatrail/tutor` and `gh release verify-asset <tag> <file> -R altnovatrail/tutor` check a release and a file you downloaded. The Go command checks the module itself against the Go checksum database.

## Documents

- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links, and what is still to decide.
- [docs/decisions.md](docs/decisions.md) — the decision log: what was decided, why, and what was rejected.
- [docs/sources.md](docs/sources.md) — the external facts the documents rest on, a provider's terms, a price, a law, each with its address, the date it was read and a quote.
- [docs/measurements.md](docs/measurements.md) — the measurements: what was measured, how, on what, and the numbers.
- [RUN.md](RUN.md) — the work plan, in epics.
- `run/` — the plan's other tables: [the epics for later](run/EPICS.md), [the open questions](run/QUESTIONS.md) and [the technical debt](run/DEBT.md).

The working rules, for people and coding agents alike, are in [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md). The platform's shared vocabulary, formats, contracts and decisions are in [spec](https://github.com/altnovatrail/spec), checked out here as a git submodule: clone this repository with `git clone --recurse-submodules`.
