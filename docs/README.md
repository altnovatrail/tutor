# tutor — the documents

> The index of `docs/` and what the repository's [README](../README.md) leaves out: the releases, how to contribute, and where building and running are described. README holds only what the component is and its place in NovaTrail (R132).

## Documents

- [architecture.md](architecture.md) — the component: what it does and does not do, its data and links (section 1), what is still to decide (section 2), and how it is built (section 3).
- [decisions.md](decisions.md) — the decision log: what was decided, why, and what was rejected.
- [sources.md](sources.md) — the external facts the documents rest on — a provider's terms, a price, a law —, each with its address, the date it was read and a quote.
- [measurements.md](measurements.md) — the measurements: what was measured, how, on what, and the numbers.
- The sheet plans and the drawn pages of eval: [sheets.md](sheets.md), [free-sheets.md](free-sheets.md), [near-sheets.md](near-sheets.md), [workbook.md](workbook.md) and [essays.md](essays.md).
- [plan/](plan/) — the archives of the done epics and of the answered questions, in Russian.
- [RUN.md](../RUN.md) — the work plan, in epics, and `run/` — its other tables: [the epics for later](../run/EPICS.md), [the open questions](../run/QUESTIONS.md) and [the technical debt](../run/DEBT.md).

## Releases

A release comes out by itself after each merge into `main` that changes what the module is built from, and its number is the next patch of line 0.1 (R62; architecture 3.2, "Release"). It is the tag of the Go module, which api builds from — `go get github.com/altnovatrail/tutor@vX.Y.Z` —, and its files: the source of the commit, `tutor_<version>_source.tar.gz`; the open views of the measurement's tasks, `tutor_<version>_open-tasks.json`, a copy of `eval/open-tasks.json`, which canvas's CI puts into the build of its samples page by the release's tag and the file's sum (R64; architecture 3.1, "Format of the file of open views"); and `SHA256SUMS` over them. Each file comes with its Sigstore bundle, `<file>.sigstore.json`.

The releases are immutable. Inside NovaTrail a file of a release enters code or CI only once `just -f spec/justfile release-verify` has accepted it (RULES.md spec, Hard rules). Outside it, `gh release verify <tag> -R altnovatrail/tutor` checks a release and `gh release verify-asset <tag> <file> -R altnovatrail/tutor` a file downloaded from it. The Go command checks the module itself against the Go checksum database.

## Contributing

The working rules, for people and coding agents alike, are in [CLAUDE.md](../CLAUDE.md), with the rules every NovaTrail repository shares, spec's RULES.md, which it imports, and [AGENTS.md](../AGENTS.md) for an agent other than Claude Code. How a change comes in — an issue first, then a pull request whose commits are signed off under the DCO — is in the organization's [CONTRIBUTING.md](https://github.com/altnovatrail/.github/blob/main/CONTRIBUTING.md); a vulnerability is reported privately, as the [security policy](https://github.com/altnovatrail/tutor/security/policy) says. Issues and pull requests are in English, a person's too (spec's RULES.md, Language).

## Build and run

All work happens in the devcontainer, with the `spec` submodule checked out: clone the repository with `git clone --recurse-submodules` and open it in its container (CLAUDE.md, Environment). The recipes are in the [justfile](../justfile), and `just --list` names each. What checks each part, and what a task that touches it runs, is the part's rule: [go](../.claude/rules/go.md) for the Go code, [eval](../.claude/rules/eval.md) for the eval runner `tutor-eval`, its sets, where each command writes and reads, and the paid runs, and [ci](../.claude/rules/ci.md) for CI, the release and the devcontainer. The packages and the recipes are architecture 3.2; the check board and its record mode, architecture 3.3; the commands of eval, architecture 3.1 and 3.4.
