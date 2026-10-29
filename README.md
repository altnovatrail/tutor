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

A component of [NovaTrail](https://github.com/altnovatrail/spec/tree/v0.3.2), a universal open platform for adaptive learning, interactive lessons and AI tutoring. It belongs to stage 2, First slice with learners, with canvas, api, app, skill-map and infra; together with canvas it brings the stage's first code, which for tutor is the check board below.

## Status

The MVP is the check board: you solve eleven fixed maths tasks by hand in the browser and press Check at any step, before you finish too. Five are solved in lines; the other six, tasks 6 to 11, in free form: on a number line, with a drawing, or with the working at the side. The Claude models you tick read the work, each at the effort you choose, and Claude Haiku 5.5 and Sonnet 5.5 without thinking if you like. Each gives only a verdict: correct, no mistakes so far, an error, which the model marks on the board in its own colour, or cannot read. You mark whether each was right. Each answer shows its time, when its first token came, its price and its tokens, thinking included; the summary shows, for each model at each effort, how often it was right and what an answer costs and takes. On 2026-10-23 Claude Opus 5.5 at high effort judged all five tasks the board then had right, at about a cent and five seconds a check (R20 in [docs/decisions.md](docs/decisions.md)); GPT-6 Astra, which the board can also ask, is not used. The reading measurement of epic E2 asked one question: does the writing process, stroke order and erasures, help Claude judge a finished sheet? On 40 sheets, 39 of them drawn by code with the solution in lines from the top down, it did not show any help: Claude Opus 5.5 judged every sheet right from the picture alone (R27). Epic E4 (R28) came next and was done on 2026-10-28: solutions in free form, with a number line, a drawing or notes at the side, then reading the work into an observation with and without the key, hints, workbook cells and essays. It started with a pilot on this board, and spec recorded its conclusions in v0.3.2 (R49 in [docs/decisions.md](docs/decisions.md)). Its extended measurement follows in the same epic (R50): more Claude configurations, hundreds of sheets, pages and essays that code draws, and ten sheets the author writes messily. Start the board in a terminal of the devcontainer:

```
just board
```

It asks for the Anthropic and OpenAI keys for eval without showing them, so no key lands in the shell's history; Enter skips a model. When the terminal says the board is at http://127.0.0.1:8080, open that address, or the one the Ports tab of VS Code shows if it forwarded the port under another number. Until then the page only keeps loading: VS Code forwards the port, but nothing answers on it yet.

The measurement's sheets are written in the board's record mode, which asks no model and so needs no key:

```
just record
```

Pick a sheet of the plan, write what it shows with a pen or a mouse, press Done, check the markup the plan filled in and press Save. If the board grouped the strokes into lines wrongly, pick strokes on the sheet and move them to another line, or join two lines. The board checks the markup after every change, and its words show beside the fields. On a sheet with a planned erasure, the board takes the move of the eraser on the plan's line for it, and you check that too. The sheet is saved with every stroke's points, their time and pressure, and every erasure, as a sample in `samples/sheets`, outside git.

The three free-form sheets of epic E4 that you write yourself are made of parts: a number line, a drawing, notes at the side. `just free-record` starts the same mode on them and saves them in `samples/free`. After Done the board frames each group of strokes as a part, in the order you wrote them. A drawing you drew on after a pause comes in two frames, so join them; to move strokes to another part, drag a rectangle around them to pick them.

The sheets you do not write, code draws: `go run ./cmd/tutor-eval draw` draws every sheet of the plan that has no file yet, saves it beside yours with the writer `synthetic`, and leaves yours as they are. With `-png DIR` it also saves, in DIR, the final picture of each sheet it draws and its frame before each erasure. For epic E4, code draws into `synthetic/`, out of git too. No handwriting is in it, so coding agents work with it and run the measurements on it themselves, and the board never writes there: your handwriting stays in `samples/`.

Once the sheets are written and split into a development and a test part (`tutor-eval sheets`, then `tutor-eval split` once), the measurement runs on a part:

```
just measure -part dev
```

The measurement of free writing of epic E4 runs the same way on the 40 sheets code draws, `just free -part dev`, in the judge's prompt for free writing; `-part all` runs every sheet of a plan without the split, as for your three sheets. Either run first says how many calls it makes and what they are expected to cost, and starts only when you type yes. It starts no call that could take it past its cap, $3 for the development part, $6 for the test and $2 for all the sheets, and logs every answer in `runs` beside the folder of the sheets, outside git. With `-stub` it asks no model: a stub answers by the markup, and the run needs no key. For the extended measurement, `go run ./cmd/tutor-eval gen sheets|near` makes a bank of tasks and a plan of their sheets with their truth by construction, as new files, and `gen workbook|essays` the pages of the workbook or the essays, and `draw -shift` leans the answer's digit of a near sheet as far as it says. `run -kind essay -two` assesses an essay in two calls, reading it and then assessing the text read. `-readers` names the Claude configurations a run asks: the three of E4 by default, and for the extended measurement four more, Opus 5.5 at medium, Sonnet 5.5 at low with thinking and without, and Haiku 5.5 at medium (R50). For it you write ten more sheets in free form, messily on purpose, with `just messy-record`: they go to `samples/free-messy`, out of git. `go run ./cmd/tutor-eval report LOG` then turns the log into the report, asking no model: whether the writing process helps each judge, by the criterion of section 3.1 of [docs/architecture.md](docs/architecture.md), and the other numbers, in Markdown and CSV beside the log.

The workbook of epic E4 is 20 pages that code draws in memory from `eval/workbook.json`: ten items a page, each with its boxes, and in each box a value written by hand, plainly or with a digit leaning toward its pair, mirrored or crossed out and written again. Its pages are listed in [docs/workbook.md](docs/workbook.md), and `go run ./cmd/tutor-eval workbook -png synthetic/png/workbook` saves the crop of every filled box, what a reading sees. The essays of epic E4 are 20 short essays in English with one rubric of six criteria, and whether each essay meets each criterion as planned: `eval/essays.json`, listed in [docs/essays.md](docs/essays.md). Code draws them in memory too, and `go run ./cmd/tutor-eval essays -png synthetic/png/essays` saves the crop of every page.

How it works is section 3.3 of [docs/architecture.md](docs/architecture.md), and the plan of epic E4's measurements is its section 3.4. The plan is [RUN.md](RUN.md); the author picks the next epic, and detailing, the first epic, comes first in its table 2.

## Documents

The documents are in Russian.

- [docs/architecture.md](docs/architecture.md) — the component: what it does and does not do, its data and links, and what is still to decide.
- [docs/decisions.md](docs/decisions.md) — the decision log: what was decided, why, and what was rejected.
- [RUN.md](RUN.md) — the work plan, in epics.

The working rules, for people and coding agents alike, are in [CLAUDE.md](CLAUDE.md) and [AGENTS.md](AGENTS.md). The platform's shared vocabulary, formats, contracts and decisions are in [spec](https://github.com/altnovatrail/spec/tree/v0.3.2), checked out here as a git submodule at `v0.3.2`: clone this repository with `git clone --recurse-submodules`.

## License

[Apache-2.0](LICENSE).
