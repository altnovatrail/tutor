# tutor

[![CI](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml)
[![CodeQL](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/altnovatrail/tutor)](go.mod)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/altnovatrail/tutor/badge)](https://scorecard.dev/viewer/?uri=github.com/altnovatrail/tutor)
[![License](https://img.shields.io/github/license/altnovatrail/tutor)](LICENSE)

[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=code_smells)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)

The AI side of the NovaTrail tutor: it reads the learner's work, handwriting or text, into an observation, reviews the working for the first mistake, assesses an essay against the module author's rubric, and words the hint for the rung of the hint ladder that the platform's program chose. It holds the platform's prompts, one per AI capability and each with its own eval set, the adapters to AI providers, the exact models admitted for each capability, and the one function that admits an AI route for the learner's age category before every request; it reports each call's usage, by which api counts the platform AI quotas, and its eval runner is a CLI on the same code. It is a library inside api: it decides no verdict, picks no lesson or task and stores nothing, and its requests carry no identifier of the learner or the account: on the own-key route only the key itself goes along, to its own provider, which ties each call to the key owner's account.

A component of [NovaTrail](https://github.com/altnovatrail/spec/tree/v0.2.32), a universal open platform for adaptive learning, interactive lessons and AI tutoring. It belongs to stage 2, First slice with learners, with canvas, api, app, skill-map and infra; together with canvas it brings the stage's first code, which for tutor is the check board below.

## Status

The MVP is the check board: you solve five fixed maths tasks by hand in the browser and press Check at any step, before you finish too. The Claude models you tick read the work, each at the effort you choose, and Claude Haiku 5.5 and Sonnet 5.5 without thinking if you like. Each gives only a verdict: correct, no mistakes so far, an error, which the model marks on the board in its own colour, or cannot read. You mark whether each was right. Each answer shows its time, when its first token came, its price and its tokens, thinking included; the summary shows, for each model at each effort, how often it was right and what an answer costs and takes. On 2026-10-23 Claude Opus 5.5 at high effort judged all five tasks right, at about a cent and five seconds a check (R20 in [docs/decisions.md](docs/decisions.md)); GPT-6 Astra, which the board can also ask, is not used. Next comes the reading measurement of epic E2, on Claude models only and narrowed to one question: does the writing process, stroke order and erasures, help the model judge a solution (R21, R24)? The board sends the model only the picture. For the measurement it will also record about 40 of the author's sheets with their strokes and timing, and Claude will judge each sheet both from the picture alone and with the writing process. Start the board in a terminal of the devcontainer:

```
just board
```

It asks for the Anthropic and OpenAI keys for eval without showing them, so no key lands in the shell's history; Enter skips a model. When the terminal says the board is at http://127.0.0.1:8080, open that address, or the one the Ports tab of VS Code shows if it forwarded the port under another number. Until then the page only keeps loading: VS Code forwards the port, but nothing answers on it yet.

The measurement's sheets are written in the board's record mode, which asks no model and so needs no key:

```
just record
```

Pick a sheet of the plan, write what it shows with a pen or a mouse, press Done, check the markup the plan filled in and press Save. The sheet is saved with every stroke's points, their time and pressure, and every erasure, as a sample in `samples/sheets`, outside git. A sheet with a planned erasure can be saved once the board marks erasures.

How it works is section 3.3 of [docs/architecture.md](docs/architecture.md). The plan is [RUN.md](RUN.md); detailing, the first epic, waits for the measurement's conclusions.

## Documents

The documents are in Russian.

- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links, and what is still to decide.
- [docs/decisions.md](docs/decisions.md) — the decision log: what was decided, why, and what was rejected.
- [RUN.md](RUN.md) — the work plan, in epics.

The working rules, for people and coding agents alike, are in [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md). The platform's shared vocabulary, formats, contracts and decisions are in [spec](https://github.com/altnovatrail/spec/tree/v0.2.32), checked out here as a git submodule at `v0.2.32`: clone this repository with `git clone --recurse-submodules`.

## License

[Apache-2.0](LICENSE).
