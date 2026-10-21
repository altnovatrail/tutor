# tutor

The AI side of the NovaTrail tutor: it reads the learner's work, handwriting or text, into an observation, reviews the working for the first mistake, and words the hint for the rung of the hint ladder that the platform's program chose. It holds the platform's prompts, one per AI capability and each with its own eval set, the adapters to AI providers, the exact models admitted for each capability, and the one function that admits an AI route for the learner's age category before every request; it reports each call's usage, by which api counts the platform AI quotas, and its eval runner is a CLI on the same code. It is a library inside api: it decides no verdict, picks no lesson or task and stores nothing, and its requests carry no identifier of the learner or the account: on the own-key route only the key itself goes along, to its own provider, which ties each call to the key owner's account.

A component of [NovaTrail](https://github.com/altnovatrail/spec/tree/v0.2.9), a universal open platform for adaptive learning, interactive lessons and AI tutoring. It belongs to stage 2, First slice with learners, with canvas, api, app, skill-map and infra; together with canvas it brings the stage's first code, a measurement of how well AI reads the learner's work.

## Status

The reading measurement, epic E2 of [RUN.md](RUN.md), is planned and comes first. Detailing, the first epic, which decides what the component's card left open, waits for the measurement's conclusions. There is no code here yet: it starts with that measurement.

## Documents

The documents are in Russian.

- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links, and what is still to decide.
- [docs/decisions.md](docs/decisions.md) — the decision log: what was decided, why, and what was rejected.
- [RUN.md](RUN.md) — the work plan, in epics.

The working rules, for people and coding agents alike, are in [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md). The platform's shared vocabulary, formats, contracts and decisions are in [spec](https://github.com/altnovatrail/spec/tree/v0.2.9), checked out here as a git submodule at `v0.2.9`: clone this repository with `git clone --recurse-submodules`.

## License

[Apache-2.0](LICENSE).
