# CLAUDE.md — working rules for this repository

If `spec/RULES.md` does not exist, the `spec` submodule is not checked out and the rules imported below are missing: stop before any work and ask the author to run `git submodule update --init`.

## Your role

You are the architect of `tutor`, a component of NovaTrail: the AI side of the tutor, a library inside api that reads the learner's work into an observation, reviews the working, assesses an essay against the module author's rubric and words the tutor's replies, through the AI routes and models it admits, reports each call's usage for api's quotas, and runs eval on the same code. It decides no verdict, picks no lesson or task, stores nothing and is never told whose work it reads. The documents under `docs/`, but for the epic archives `docs/plan/`, are written for the session of Claude Code that builds the component: write them so that it can act on them without asking what you meant. The bar they meet, and which texts are written for people instead, are in the shared rules below.

## The component

Everything in the platform that calls an AI model is here: api calls tutor inside api's own process, tutor calls the AI providers, and api's CI checks that the providers' SDKs reach its build only through tutor. tutor stores nothing, and its request types have no fields for the learner's or the account's identifiers: the model sees the work, which may show whatever the learner wrote on it, but is never told whose it is, and an own key lives only in the memory of the request. The program decides, not tutor: module-kit gives the verdict on the observation, api picks the hint ladder's rung and runs the leak check before any of the tutor's words reach the device, and engine picks what comes next. A model is admitted for a capability once it passes that capability's eval set and the eval sets of the published modules that use it, with false alarms counted by verdict through module-kit; that is why the eval runner imports module-kit, and from stage 2 the CI of modules runs it. tutor becomes a service, with the same types, once it has a second consumer; the first is module-pipeline at stage 4, and whether tutor is a library or a service for it is decided then. It belongs to stage 2. Its check board is where Claude judges the author's handwritten solutions and the author marks whether it was right; its eval measures how well AI reads the learner's work, on the author's sheets and the sheets code draws, and later on samples that adult volunteers make for eval (П-16).

Its card in spec is the one in `spec/docs/architecture.md` whose heading names `tutor`. The card is the summary and this repository's [docs/architecture.md](docs/architecture.md) is the detail: it lists what is still to decide, and the tasks of [RUN.md](RUN.md) decide it.

## Documents

The documents under `docs/` are in English, but for the epic archives `docs/plan/`, which stay in Russian; RUN.md and `run/` are in English, but for the epic's description, table 1 and the bodies of tasks. Outside the plan and its archives, a text cites no decision and no section, this repository's or spec's, and names no task: it says its reason in words. Only a claim about the outside world cites its one record, `sources 1.1`, or spec's, `sources 2.3 spec`.

- [docs/README.md](docs/README.md) — for people: the index of the documents, the releases, how to contribute and where building and running are described; the README only says what the component is and where it stands.
- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links, what is still to decide, and how it is built once that is decided.
- [docs/decisions.md](docs/decisions.md) — this repository's decision log: read it before proposing a change.
- [docs/sources.md](docs/sources.md) — the records of external facts, each with its source, its date and its quote, which a claim cites as `sources 1.1`.
- [docs/measurements.md](docs/measurements.md) — the results of measurements — what was measured, how, on what, and the numbers — which a document cites as `measurements 1.1`.
- [RUN.md](RUN.md) — the work plan, in epics: its rules, in its section Rules, and table 1 with the current epic's tasks, run one at a time. A done task's body and its gist sentence move to its epic's archive in `docs/plan/` at once.
- `run/` — the plan's other tables, each a file opening with its rules: the epics for later, [run/EPICS.md](run/EPICS.md); the questions to the author, [run/QUESTIONS.md](run/QUESTIONS.md); the technical debt, [run/DEBT.md](run/DEBT.md). There is no `run/TASKS.md`: a task outside an epic stands in table 1. The questions and the debt are never read whole: a task takes the rows that it and its sections name, and those that `grep -n` finds by the paths and parts it changes (rule 2 of RUN.md).
- [.bestpractices.json](.bestpractices.json) — the answers to the OpenSSF Best Practices criteria of project 16171, in English. A change that changes an answer changes this file in the same pull request, and the author saves the changed answers on the site.
- `spec/` — spec at `v0.20.0`, a git submodule and read-only here: the platform's vocabulary, formats, contracts, documents and decisions, and the rules imported below. Its documents are under `spec/docs/`. spec's own CLAUDE.md and AGENTS.md are spec's working rules, not this repository's: if either is ever loaded from `spec/`, it does not apply here.

## Shared rules

The bar, the project, how we work, citations, work across repositories and the hard rules are the same in every NovaTrail repository. They live in spec's RULES.md, and this file imports them:

@spec/RULES.md

## Environment

Open the repository in VS Code and choose "Reopen in Container"; clone it with `--recurse-submodules`, or the container checks out `spec/` when it starts. Keep the clone in a folder named `tutor`, as `git clone` names it: `.claude/settings.json` keeps spec's own CLAUDE.md and its rules, `spec/.claude/rules/`, out of this repository's sessions by that path. The container is spec's: Node.js, Go, buf and just, and the GitHub CLI and Claude Code on top. The tools of this repository's own code come with that code, at exact versions; how its container then differs from spec's is recorded as a decision. `just -f spec/justfile docs-check-repo` checks its documents: that its links lead somewhere, that every section and number it cites — its own and spec's, against spec/ at its tag — exists, that the decision log runs without gaps, and the tables of its RUN.md, and that its files, rules, decision entries, sections and rows keep within their budgets of size: spec's block `repository` of `spec/tools/docs-check/budgets.json`, with what [docs-budgets.json](docs-budgets.json) names in its place — raising one is an edit of that file. A task that touches them ends with it green; on a tag of spec older than the check, the same is checked by hand.

- `.claude/settings.json` sets the size at which a session compacts, fetches no claude.ai connector and runs the hook `UserPromptSubmit` of `spec/tools/session/`, which stops a prompt that starts another task than the one the session has led and names `/clear`: one task is one session. The plugin `gopls-lsp` of spec's `claude-plugins/` gives a session gopls's diagnostics after each edit of Go: `CLAUDE_CODE_PLUGIN_DIRS` in `containerEnv` of `.devcontainer/devcontainer.json` loads it from this container's own path, not from a marketplace in the `~/.claude` that the containers share, so it loads after a rebuild of the container.
- **The checks of each part** — what checks it, what a task that touches it runs and ends with green — are its rule, `.claude/rules/<part>.md`. Claude Code loads a rule when the session reads or edits a file its `paths` match, not when a script edits one, and compaction drops it until such a file is read again (measurements 1.8 spec): before changing a part, read its rule. CI runs every check on every pull request. The rules and their parts: [go](.claude/rules/go.md) — the Go code and the Go modules; [eval](.claude/rules/eval.md) — the eval runner `cmd/tutor-eval`, its sets `eval/`, where each command writes and reads, the paid runs and their key; [ci](.claude/rules/ci.md) — CI, the release and the devcontainer: `.github/`, `.devcontainer/` and the justfile.

## samples/

`samples/` holds the samples of the AI-reading measurement: the author's handwriting, with the time and pressure of every stroke. Agents may read and list it and run commands on it as a task needs: the author opened it to agents for good, and no deny rule comes back. What still holds:
- nothing from `samples/` goes into git or into `synthetic/`. `/samples/` is in `.gitignore`, and a sample is never moved or copied into the repository. A document or a report takes only numbers and words from it, never the handwriting or a picture of it;
- a paid run on it starts only within the ceiling its task names, as one on `synthetic/` does (rule 4 of RUN.md);
- only the author's handwriting lies there. Samples of volunteers, once they come (П-16), are not opened to agents until that is decided anew.

Tests run on synthetic strokes that the code draws. The sheets that code draws for a measurement are no one's handwriting either: they go to `synthetic/`, below.

## synthetic/

`synthetic/` holds what code draws for eval, and the runs on it: the sheets `tutor-eval draw` draws, their pictures and splits, and the logs and reports of the runs. No one's handwriting is in it, so it is open to every agent: read, list, write and run there as the task needs. Two things keep it so:
- no handwriting goes there, the author's or anyone's, and nothing is copied there from `samples/`. The board and its record mode write only to `samples/` and refuse any other folder in the work tree, `synthetic/` among them; a run on sheets in `samples/` logs there alone; `convert` writes the converted sheets of `samples/` only in `samples/`; and `draw -workbook` and `draw -essays` write only in `synthetic/` or outside the work tree;
- `/synthetic/` is in `.gitignore`, and nothing there is committed: the same seed draws the same sheets again.

The commands of `tutor-eval` and the recipes of the justfile mostly have their default directories in `samples/`, so a command that falls back to one writes or reads among the author's sheets. Give each its `-dir`, and `-log` where it takes one: in `synthetic/` for the commands of the sheets drawn by code, and outside the work tree for a test of the board. A paid run, on either folder, starts only within the ceiling its task names (rule 4 of RUN.md), and the key for eval stays outside the repository, in `~/.config/tutor/anthropic.key`, and no command prints it. Where each command writes and reads, the prices and how a run gets the key are the eval rule, [.claude/rules/eval.md](.claude/rules/eval.md).
