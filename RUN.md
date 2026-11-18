# RUN.md — work plan of tutor

What tutor does, does not do and has still to decide — [docs/architecture.md](docs/architecture.md); why — [docs/decisions.md](docs/decisions.md). The platform is in `spec/`, a submodule at tag v0.18.34: the component's card — architecture 3.6 spec; the order of repositories — repositories 6 spec; learners' data and law — privacy spec; the platform's decisions — spec's log. Small tasks, one at a time, gathered in epics; only the current epic is in detail, or two when a second runs beside it (R04; R19 spec). Other tables, each a file opening with its rules (R153 spec): 2, epics for later — [run/EPICS.md](run/EPICS.md); 3, questions to the author — [run/QUESTIONS.md](run/QUESTIONS.md); 4, debt — [run/DEBT.md](run/DEBT.md). No `run/TASKS.md`: a task outside an epic stands in table 1 (R103, R116).

Language (R154 spec): rules and tables in English, terse, for the session; the epic's description, table 1 and the bodies of tasks in Russian — the author writes and reviews them.

## Rules

A task starts with «Выполни задачу T01 из RUN.md» in a new session in the devcontainer. One task — one session (R143 spec): the hook `UserPromptSubmit` of [.claude/settings.json](.claude/settings.json) — `spec/tools/session/` — stops «Выполни задачу T…» or «планируй T…» of another task in the same session and names `/clear`; the same task passes.

1. Do only the named task; touch other tasks' files only where it requires.
2. Read first: [CLAUDE.md](CLAUDE.md) with the rules it imports (RULES.md of `spec/`), the task, the sections it cites — its own and spec's —, [docs/decisions.md](docs/decisions.md).
   - Conflict with the documents or something missing — stop and ask; never guess.
   - A gap in the component itself — a subsection of [docs/architecture.md](docs/architecture.md), section 2. A gap across repositories — a row of [run/QUESTIONS.md](run/QUESTIONS.md) whose question opens with the mark «В реестр spec» (R81 spec).
   - From [run/DEBT.md](run/DEBT.md) and [run/QUESTIONS.md](run/QUESTIONS.md) take only the rows the task and its sections name and those `grep -n` finds by the paths and parts it changes (R143 spec).
3. Devcontainer only, with `spec/` in place — RULES.md, Hard rules; if `spec/` is empty — CLAUDE.md.
4. Outside this repository — editing `spec` or other repositories, their settings, anything that costs money — only after a warning (what, where, cost) and the author's confirmation.
   - Exception, no confirmation: the task's checks by R146 spec. AI calls on the stub: the tests' fake adapters and `-stub` of the runner `cmd/tutor-eval`, with no key. `tutor`'s one path in a browser is the board; a container of the kind `go` has no Playwright, so the executor checks the board without a browser: by its tests, and by `go run ./cmd/tutor-eval board` with `-dir` outside the work tree and a fake key in `TUTOR_ANTHROPIC_API_KEY`, by requests to its server. `just board` is the author's: it takes the real key and writes to `samples/` (R110).
   - A real AI call only where the task checks the model's answers — the words, the reading, the assessment —, that is, by an eval run (R28): on the eval key, `~/.config/tutor/anthropic.key`, under its own spend limit at the provider, set by the author, and within the ceiling the task's text names. With no ceiling in the task, a real call waits for confirmation, like anything else that costs money. «Сделано» of such a task names what was spent: the provider, the number of calls and the sum.
   - Nothing on devices: `tutor` has no learner's screens, and the board is the author's tool.
5. External facts and measurements — as RULES.md, How we work, says.
6. End with the checks of what the task changed: documents — `just -f spec/justfile docs-check-repo` (CLAUDE.md, Environment); code — the rule of its part. Then `/code-review high`, one round; a small change may skip the review.
7. Do not commit: the author commits after review, unless the author's word says otherwise, as «доведи до мержа». A run of spec's orchestrator, «Сделай эпик E…», is such a word on each of its tasks: its executor works by the contract `spec/tools/orchestrator/executor.md` — commits, pushes and opens the pull request, never merges (R156 spec; R115).
8. End with a short report in Russian: what was done, files, how to check, what the review found or why there was none, what is open and what goes «в реестр spec».
9. When done: table 1 `[ ]` → `[x]`; append to the body «Сделано (дата)» — one line, what was done and its pull request, which says how it was checked; where the author commits after review, the report says it. What stays open is rows, not text: a question for the author, of [run/QUESTIONS.md](run/QUESTIONS.md); what is put off, of [run/DEBT.md](run/DEBT.md) (R160 spec).
   - Then move the body verbatim, relative links fixed, to the epic archive `docs/plan/<epic>.md`, with a gist sentence — an unheaded list item at the start of the receiving section: «T01 — сделана <дата>: <итог одной фразой>». Neither stays here, and a search by heading finds one body, in the archive (R153 spec; R23).
10. Language (R14 spec, R42 spec, R143 spec, R154 spec): this plan and `run/` in English; the epic's description, table 1, the bodies of tasks, the epic archives `docs/plan/` and the reports in Russian; `docs/` in English; README, CLAUDE.md, AGENTS.md, code, comments, schema descriptions, prompts and normative texts for authors in English; commit messages and a pull request's title and description in English and terse, for the session, never a copy of the report.
11. A task larger than one review (≈ 30–60 minutes of reading): propose a split before starting.
12. Before a new task, close the findings on the previous one (R30 spec); an allowed departure from rule 1.
    - SonarCloud — [New Code](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor), on the task already merged; CodeQL and Scorecard — `gh api 'repos/altnovatrail/tutor/code-scanning/alerts?state=open'`.
    - Fix in code first; "false positive" is the last resort, with the reason in words: shared reasons — R33 spec, own ones — in the log. Never skip silently.
    - Report these fixes in a separate paragraph: the author commits them apart from the task.

**Statuses:** `[ ]` not started; `[x]` done; `[-]` not done, reason in the task; `[?]` deferred, reason and when to return in the task. A task that fails the author's review returns to `[ ]` with what to fix.

**Who:** «Claude» — the executor does it, the author reviews; «Claude, проверяю я» — texts the author reads line by line; «я + Claude» — the author does some steps: decisions, sign-ins, GitHub settings, confirmations by rule 4.

**Plan:** «да» — plan mode first: decisions missing from the documents, or larger than one review (rule 11); «—» — run at once.

**Task body** (Russian): «Цель», «Сделать», «Как проверить», «Разделы»; as needed «Не делать», «Решить при планировании», «Мой шаг», «Стоимость», «Риски»; from planning «Почему», «Решено автором (дата)», «Выяснилось при планировании»; when done «Сделано (дата)» (rule 9).

## Epics

- **Table 1** is the current epic, or two (R04): its tasks in detail, run one at a time.
- **No tasks outside an epic** (R103): a task that does not come from an epic — a move to a tag of `spec`, an analysis by R145 spec, an instruction of the author — stands in table 1 of the current epic; with none, it is the first task of the next, as T145 started E9 (R115).
- **Next epic.** Once table 1 is all checked, the author picks the next epic from [run/EPICS.md](run/EPICS.md) and splits it with Claude in plan mode into tasks of one review each, into table 1 and this file.
- **A done epic** goes to `docs/plan/<epic>.md` (R59 canvas-api): bodies and gist sentences are there already (rule 9); move its description and table of tasks verbatim; questions and debt stay in `run/`; until the next epic, table 1 reads «Current epic — none» and names the last done epic. Measure the epic's sessions — `just -f spec/justfile tokens-report --since <first day of the epic> --row` — as a row of measurements 4.1; a new analysis by the threshold of R145 spec.
- Never read an archive whole: find a task by heading — `grep -n '^#### T03\.' RUN.md docs/plan/*.md`; from another repository — «T03 tutor». Task numbers are global, never reused, and taken free at the moment of writing.

## Table 1. Current epic — E19. Тексты без истории (R160 spec)

**Результат:** `tutor` пишет по R160 spec: его документы, план и комментарии кода держат текущее состояние, а не историю; `docs-check-repo` проверяет предел записи журнала 12 КБ и номера решений в комментариях.

**Основа:** R160 spec, R32 spec, R145 spec, R159 spec; RULES.md spec — The bar, How we work, Across repositories; строка 7.32 COORDINATION.md spec — правило эпика E16 `spec` по тексту T206 `spec`; R76 spec, R156 spec; R103. Задачу T170 дал оркестратор `spec`; текущего эпика не было, и по R103 она — первая задача следующего. Она больше одного ревью, и по правилу 11 разделена: T170 — переход на тег, `docs-budgets.json` и план; T171–T181 — документы и комментарии, каждая — одно ревью. Тела сделанных задач и их фразы итога — в архиве [docs/plan/E19.md](docs/plan/E19.md).

**Документы (T171–T175)** — по R160 spec, «One home» и «A done task»:
- без хроник версий, статусов и повышений, дат «с такого-то дня», номеров задач, которые что-то сделали, и пересказа того, как было раньше: история — в git, релизах и архивах `docs/plan/`; пример — абзац «Status» `docs/architecture.md` `infra`;
- значение, заданное в файле, — версия, тег, цена в коде, модель, — документ не повторяет, а называет файл: `go.mod`, подмодуль `spec`, `ARG` `.devcontainer/Dockerfile`, `admission.Admitted`;
- архитектура описывает текущее устройство и ссылается на решение номером, не пересказывая его — почему и что отвергнуто, есть в записи;
- ссылка на поправку записи `spec`, которую тег переписал на месте (R143, R145, R154, R155 spec и те, что перепишут задачи E16 `spec`), — на саму запись: `R143 spec`, а не «the 2026-11-15 amendment to R143 spec»; ссылки на поправки своих записей остаются, пока запись не переписана;
- вне правила — `docs/sources.md`, `docs/measurements.md` и `docs/plan/`; что из раздела уходит, если это замер или внешний факт, — в них, а не в никуда;
- раздел стал меньше — его предел в `sections` `docs-budgets.json` понизить до размера с округлением вверх до КБ или убрать, если раздел в пределе 24 КБ `spec`;
- журнал, [docs/decisions.md](docs/decisions.md), — не задача эпика: запись переписывает на месте, со свёрнутыми поправками, задача, которая меняет то, что она говорит (R160 spec); правка документа по записи запись не меняет, а правка одной ссылки в записи — ссылки на поправку, которой больше нет, — не касание записи, и её поправки остаются. Ссылки журнала на поправки R143 и R145 `spec` T170 уже свела к записям.

**Комментарии (T176–T181)** — по R160 spec, «Comments»: комментарий говорит то, чего не скажут код и имена, — почему, ограничение, ловушка, контракт экспортируемого API; без пересказа кода, истории, номеров задач (`T143`) и разделов документов (`architecture 3.1`); где причина — решение, комментарий даёт ограничение одной фразой и кончается его номером, `(R25)`; номер `spec` — `(R25 spec)`. Комментарий doc экспортируемого имени остаётся и начинается с имени (golangci-lint). Строки, литералы, промпты и тексты в `testdata/` — не комментарии: их не трогать, иначе поменяются запросы, хеши замороженных версий (`TestFrozenV1` `internal/correct` и такие же) и золотые логи. Комментарий, что держит тест, и комментарий `//nolint`, `//go:` — по их правилам. Проверка — правило [go](.claude/rules/go.md): `just lint`, `just ci-test`; для `#`-комментариев — правило [ci](.claude/rules/ci.md).

| Статус | ID | Задача | Кто | План | Зависит от |
|---|---|---|---|---|---|
| [x] | T170 | Тексты без истории (R160 spec) и тег spec v0.18.34 | Claude | — | — |
| [x] | T171 | Без истории: CLAUDE.md, README, правила частей, документы листов | Claude | — | T170 |
| [x] | T172 | Без истории: architecture 1, 2 и 3.1 | Claude | — | T170 |
| [x] | T173 | Без истории: architecture 3.2–3.4 | Claude | — | T170 |
| [x] | T174 | Без истории: architecture 3.5–3.9 | Claude | — | T170 |
| [x] | T175 | Без истории: architecture 3.10–3.12 и 4 | Claude | — | T170 |
| [x] | T176 | Комментарии: `internal/report`, `internal/runner`, `internal/measure` | Claude | — | T170 |
| [x] | T177 | Комментарии: `cmd/tutor-eval` и судьи, журнал, конвертер | Claude | — | T170 |
| [x] | T178 | Комментарии: `internal/sample`, `internal/evalsample`, `internal/synth` | Claude | — | T170 |
| [x] | T179 | Комментарии: генераторы, рисование, листы и задачи | Claude | — | T170 |
| [x] | T180 | Комментарии: корень, `admission`, провайдеры, `words`, протокол | Claude | — | T170 |
| [ ] | T181 | Комментарии: доска, подсказка, чтение, `correct`, CI и контейнер | Claude | — | T170 |

**Порядок:** T171–T181 независимы друг от друга и идут в любом порядке, по одной; T172–T175 правят один файл, разные разделы. Строка 7.32 COORDINATION.md spec ждёт всех.

#### T181. Комментарии: доска, подсказка, чтение, `correct`, CI и контейнер

- **Цель:** комментарии кода `internal/board` (с его страницей), `internal/hint`, `internal/reading` и `internal/correct`, тестов тоже, и `#`-комментарии `justfile`, `.github/`, `.devcontainer/` и `.golangci.yml` — по R160 spec, «Comments».
- **Сделать:** по «Комментариям» описания эпика; в `.devcontainer/` — только свои отличия от заготовки `spec`: строки заготовки приходят с переходом на тег.
- **Как проверить:** `just lint`, `just ci-test`; проверки правила [ci](.claude/rules/ci.md) для изменённых файлов CI; `just -f spec/justfile docs-check-repo`; `/code-review high`.
- **Разделы:** R160 spec; R32 spec; описание эпика E19.
