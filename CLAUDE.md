# CLAUDE.md — working rules for this repository

If `spec/RULES.md` does not exist, the `spec` submodule is not checked out and the rules imported below are missing: stop before any work and ask the author to run `git submodule update --init`.

## Your role

You are the architect of `tutor`, a component of NovaTrail: the AI side of the tutor, a library inside api that reads the learner's work into an observation, reviews the working, assesses an essay against the module author's rubric and words the tutor's replies, through the AI routes and models it admits, reports each call's usage for api's quotas, and runs eval on the same code. It decides no verdict, picks no lesson or task, stores nothing and is never told whose work it reads. Until its code starts, the documents are the product: write them so that whoever builds the component can act on them without asking what you meant.

## The component

Everything in the platform that calls an AI model is here: api calls tutor inside api's own process, tutor calls the AI providers, and api's CI checks that the providers' SDKs reach its build only through tutor (architecture 1 spec, architecture 3.6 spec, R15 spec). tutor stores nothing, and its request types have no fields for the learner's or the account's identifiers: the model sees the work, which may show whatever the learner wrote on it, but is never told whose it is, and an own key lives only in the memory of the request (architecture 1 spec, architecture 4.3 spec, privacy 1 spec, privacy 5.1 spec, privacy 5.2 spec). The program decides, not tutor: module-kit gives the verdict on the observation, api picks the hint ladder's rung and runs the leak check before any of the tutor's words reach the device, and engine picks what comes next (R03 spec, architecture 3.5 spec). A model is admitted for a capability once it passes that capability's eval set and the eval sets of the published modules that use it, with false alarms counted by verdict through module-kit; that is why the eval runner imports module-kit, and from stage 2 the CI of modules runs it (architecture 4.6 spec, repositories 3 spec, repositories 4 spec). tutor becomes a service, with the same types, once it has a second consumer; the first is module-pipeline at stage 4, and whether tutor is a library or a service for it is decided then (R15 spec, architecture 3.6 spec). It belongs to stage 2. Its first code is the check board, where Claude judges the author's handwritten solutions and the author marks whether it was right; the measurement of how well AI reads the learner's work follows it, on the author's sheets first and later on samples that adult volunteers make for eval (repositories 6 spec, R43 spec, П-16).

Its card in spec is architecture 3.6 spec. The card is the summary and this repository's [docs/architecture.md](docs/architecture.md) is the detail: section 2 there lists what is still to decide, and the tasks of [RUN.md](RUN.md) decide it.

## Documents

The documents under `docs/` are in Russian. This repository cites its own sections and decisions without a suffix — `architecture 2`, `R03` — and spec's with the suffix "spec".

- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links (section 1), what is still to decide (section 2), and how it is built once that is decided (section 3).
- [docs/decisions.md](docs/decisions.md) — this repository's decision log, `R01`…: read it before proposing a change. Other repositories cite its entries as `R03 tutor`.
- [RUN.md](RUN.md) — the work plan, in epics. Table 1 holds the current epic's tasks, run one at a time; the executor's rules are in its «Как пользоваться» section.
- `spec/` — spec at `v0.2.32`, a git submodule and read-only here: the platform's vocabulary, formats, contracts, documents and decisions, and the rules imported below. Its documents are under `spec/docs/`, cited with the suffix "spec". spec's own CLAUDE.md and AGENTS.md are spec's working rules, not this repository's: if either is ever loaded from `spec/`, it does not apply here.

## Shared rules

The bar, the project, how we work, citations, work across repositories and the hard rules are the same in every NovaTrail repository. They live in spec's RULES.md, and this file imports them:

@spec/RULES.md

## Environment

Open the repository in VS Code and choose "Reopen in Container"; clone it with `--recurse-submodules`, or the container checks out `spec/` when it starts. Keep the clone in a folder named `tutor`, as `git clone` names it: `.claude/settings.json` keeps spec's own CLAUDE.md out of this repository's sessions by that path. The container is spec's: Node.js and just, and the GitHub CLI and Claude Code on top. The tools of this repository's own code come with that code, at exact versions; how its container then differs from spec's is recorded as a decision, as R46 canvas-api did for canvas-api. `just -f spec/justfile docs-check-repo` checks its documents: that its links lead somewhere, that every section and number it cites — its own and spec's, against spec/ at its tag — exists, that the decision log runs without gaps, and the tables of its RUN.md (R32 spec). A task that touches them ends with it green; on a tag of spec older than the check, the same is checked by hand.

## samples/

`samples/` holds the samples of the AI-reading measurement: the author's handwriting, with the time and pressure of every stroke (R05, architecture 3.1). No agent opens it, whatever the task: it does not read, list, search, copy or run anything on what is there, and it never moves a sample into the repository. Three things hold this:
- this rule, which AGENTS.md makes binding on every agent, Codex too;
- `/samples/` in `.gitignore`;
- for Claude Code, deny rules in `.claude/settings.json` on reading it, on writing there and on any shell command whose text holds its name. That includes a command that names it only in passing, such as an edit of a document that mentions it: such an edit goes through the editing tools, not the shell.

A command that walks the whole tree, such as `grep -r` or `find .`, names no folder, and neither the deny rules nor `.gitignore` keep it out of `samples/`. Walk the tracked files instead, with `git grep` and `git ls-files`: `samples/` is never tracked.

The go command given `./...`, the tools that load packages through it (golangci-lint, govulncheck, go-licenses) and `golangci-lint fmt` are the one exception, so the recipes of the `justfile` may run as they are. They read the names in each folder, `samples/` among them, but open only `.go` files, of which there are none there, and print nothing from it (R14 in docs/decisions.md). If a `.go` file ever lands in `samples/`, the exception no longer holds.

Inside the container these are rules, not an access boundary. So whatever needs the samples, the paid runs among them, is run by the author, and only the summary report, without handwriting, reaches the repository. Tests run on synthetic strokes that the code draws.
