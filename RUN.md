# RUN.md — work plan of tutor

What tutor does, does not do and has still to decide — [docs/architecture.md](docs/architecture.md); why — [docs/decisions.md](docs/decisions.md). The platform is in `spec/`, a submodule at tag v0.19.12: the component's card — architecture 3.6 spec; the order of repositories — repositories 6 spec; learners' data and law — privacy spec; the platform's decisions — spec's log. Small tasks, one at a time, gathered in epics; only the current epic is in detail, or two when a second runs beside it (R04; R19 spec). Other tables, each a file opening with its rules (R153 spec): 2, epics for later — [run/EPICS.md](run/EPICS.md); 3, questions to the author — [run/QUESTIONS.md](run/QUESTIONS.md); 4, debt — [run/DEBT.md](run/DEBT.md). No `run/TASKS.md`: a task outside an epic stands in table 1 (R103, R116).

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
10. Language (R154 spec): this plan and `run/` in English; the epic's description, table 1, the bodies of tasks, the epic archives `docs/plan/` and the reports in Russian; `docs/` in English; README, CLAUDE.md, AGENTS.md, code, comments, schema descriptions, prompts and normative texts for authors in English; commit messages and a pull request's title and description in English and terse, for the session, never a copy of the report.
11. A task larger than one review (≈ 30–60 minutes of reading): propose a split before starting.
12. Before a new task, close the findings on the previous one (R30 spec); an allowed departure from rule 1.
    - SonarCloud — [New Code](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor), on the task already merged; CodeQL and Scorecard — `gh api 'repos/altnovatrail/tutor/code-scanning/alerts?state=open'`.
    - Fix in code first; "false positive" is the last resort, with the reason in words: shared reasons — R33 spec, own ones — in the log. Never skip silently.
    - Report these fixes in a separate paragraph: the author commits them apart from the task.

**Statuses:** `[ ]` not started; `[x]` done; `[-]` not done, reason in the task; `[?]` deferred, reason and when to return in the task. A task that fails the author's review returns to `[ ]` with what to fix.

**Who:** «Claude» — the executor does it, the author reviews; «Claude, проверяю я» — texts the author reads line by line; «я + Claude» — the author does some steps: decisions, sign-ins, GitHub settings, confirmations by rule 4.

**Plan:** «да» — plan mode first: decisions missing from the documents, or larger than one review (rule 11); «—» — run at once.

**Task body** (Russian): «Цель», «Сделать», «Как проверить», «Разделы»; as needed «Не делать», «Решить при планировании», «Мой шаг», «Стоимость», «Риски»; from planning «Почему», «Решено автором (дата)», «Выяснилось при планировании»; when done «Сделано (дата)» (rule 9).

**Citations** — RULES.md spec, Citations: a task's number stands only here, in `run/` and in `docs/plan/` (R171 spec). Own sections — `architecture 2`, own decisions — `R03`; spec's — «architecture 3.5 spec», «R18 spec», its open questions — «Т-22», «П-7», «О-2»: the repository keeps no registers; other NovaTrail repositories' decisions — «R03 api»; MathTrail's — «R44 canvas-api», in a list «canvas-api: R44, R59». A task — «T03», from another repository «T03 tutor».

## Epics

- **Table 1** is the current epic, or two (R04): its tasks in detail, run one at a time.
- **No tasks outside an epic** (R103): a task that does not come from an epic — a move to a tag of `spec`, an analysis by R145 spec, an instruction of the author — stands in table 1 of the current epic; with none, it is the first task of the next, as T145 started E9 (R115).
- **Next epic.** Once table 1 is all checked, the author picks the next epic from [run/EPICS.md](run/EPICS.md) and splits it with Claude in plan mode into tasks of one review each, into table 1 and this file.
- **A done epic** goes to `docs/plan/<epic>.md` (R59 canvas-api): bodies and gist sentences are there already (rule 9); move its description and table of tasks verbatim; questions and debt stay in `run/`; until the next epic, table 1 reads «Current epic — none» and names the last done epic. Measure the epic's sessions — `just -f spec/justfile tokens-report --since <first day of the epic> --row` — as a row of measurements 4.1; a new analysis by the threshold of R145 spec.
- Never read an archive whole: find a task by heading — `grep -n '^#### T03\.' RUN.md docs/plan/*.md`; from another repository — «T03 tutor». Task numbers are global, never reused, and taken free at the moment of writing.

## Table 1. Current epic — E21. Эпик E22 `spec`: тег v0.19.11 и ссылки по R171 spec

**2026-11-23: эпик открыт.** Задачи приходят от оркестратора `spec` — эпик E22 `spec`, пачка на ветке `e22-tutor` с одним pull request (R156 spec, R159 spec). Текущего эпика не было — E20 сделан 2026-11-20, таблица 2 пуста, — а задача вне эпика по R103 — первая задача следующего. Первая задача — переход на тег, волна 0; вторая, T184, — ссылки по R171 spec: она больше одного ревью и разбита по своему шагу 9 — сама взяла план, CLAUDE.md, правила частей и комментарии кода, а документы и журнал — задачи T185–T195 ниже. Тела сделанных задач — в архиве, [docs/plan/E21.md](docs/plan/E21.md).

**2026-11-23: тег v0.19.12.** T196 перевела `tutor` на `spec` `v0.19.12`: в нём R171 spec — строгое правило ссылок (RULES.md spec, Citations), оно заменяет узкое. Вне плана и архивов — ни номера решения, своего или чужого, ни ссылки на раздел или документ, ни датированной атрибуции; внешний факт ссылается на одну запись `sources`/`measurements`, строки Д и В и вопросы О-, П-, Т- остаются. Где «Ссылки (T185–T195)» ниже с ним расходятся, — по нему. Проход по части T184 и T185–T195 — пачка на ветке `e22-tutor-2`; задачи пачки пишет оркестратор `spec`.

**Результат:** `tutor` на теге `spec` `v0.19.12`, и его ссылки — по строгому правилу R171 spec (строка 7.44 COORDINATION.md spec).

**Основа:** R156 spec, R159 spec, R171 spec, R160 spec; RULES.md spec, Citations и Across repositories; R103.

**Ссылки (T185–T195)** — по тексту T184 ([docs/plan/E21.md](docs/plan/E21.md)), шаги 4–8, каждая задача — в своих файлах и разделах:
- ссылка стоит, только если читатель пойдёт по ней, чтобы действовать: оспоримый внешний факт — одна запись `sources`/`measurements`; решённый выбор, который выглядит изменяемым, — один номер решения, один раз в тексте; открытая работа — строка «Д…» или «В…»; остальное — убрать; цепочку — свести к одному месту, где утверждение решено; в скобке — не больше двух ссылок;
- номер задачи вне RUN.md, `run/` и `docs/plan/` — убрать, назвать файл или решение; в заголовке записи `sources`/`measurements` — тоже: запись хранит дату, не номер задачи;
- ссылка на поправку записи `spec` — на саму запись: `R07 spec`, не «the 2026-11-17 amendment to R07 spec»;
- журнал: каждая датированная поправка свёрнута в действующий текст записи, на месте; «Почему», отвергнутое и цитаты источников сохранены (R160 spec); хвост «— *Consequence:*» — файлы и разделы, которые правит изменение решения, без номеров задач и без мест, которые решение только описывают; в описании pull request — таблица «поправка → место в новом тексте»;
- смысл не менять; решение, которое при свёртке оказалось противоречивым, или ссылка, о которой есть настоящее сомнение, — строка таблицы 3 и статус `waiting`;
- ключ `"citations": true` в `docs-budgets.json` включает только T195; до неё каждая задача включает его локально и проверяет, что её файлы и разделы находок `docs-check-repo` не дают, и возвращает ключ назад;
- в «Сделано» — ссылок было и стало, по видам, командой `cite.mjs` measurements 1.18.4 spec, на своих файлах.

| Статус | ID | Задача | Кто | План | Зависит от |
|---|---|---|---|---|---|
| [x] | T183 | Переход на тег `spec` v0.19.11 | Claude | — | — |
| [x] | T184 | Ссылки по правилу R171 spec: план, CLAUDE.md, правила частей, комментарии кода | Claude | да | T183 |
| [x] | T196 | Переход на тег `spec` v0.19.12 | Claude | — | T184 |
| [ ] | T185 | Ссылки: README и architecture 1–3.2 | Claude | — | T196 |
| [ ] | T186 | Ссылки: architecture 3.3–4 | Claude | — | T185 |
| [ ] | T187 | Ссылки: документы листов и sources | Claude | — | T186 |
| [ ] | T188 | Ссылки: measurements 1–2 | Claude | — | T187 |
| [ ] | T189 | Ссылки: measurements 3.1–3.10 | Claude | — | T188 |
| [ ] | T190 | Ссылки: measurements 3.11–5 | Claude | — | T189 |
| [ ] | T191 | Журнал по R171 spec: начало и записи до «Checking line by line» | Claude | — | T190 |
| [ ] | T192 | Журнал по R171 spec: от «The check board» до «Move to `spec` `v0.2.43`» | Claude | — | T191 |
| [ ] | T193 | Журнал по R171 spec: от «Reading into an observation» до «OpenSSF Best Practices» | Claude | — | T192 |
| [ ] | T194 | Журнал по R171 spec: от «The return of epic E3» до «The admission of words» | Claude | — | T193 |
| [ ] | T195 | Журнал по R171 spec: от «Move to `spec` `v0.14.0`» до конца; ключ `citations` | Claude | — | T194 |

**Порядок:** T183 и T184 — пачка на ветке `e22-tutor`, pull request #215; T196, проход по части T184 и T185–T195 — пачка на ветке `e22-tutor-2`, один pull request (R159 spec), по «Зависит от».

#### T185. Ссылки: README и architecture 1–3.2

- **Цель:** [docs/README.md](docs/README.md) и разделы 1, 2, 3.1 и 3.2 [docs/architecture.md](docs/architecture.md) ссылаются по R171 spec.
- **Сделать:** по «Ссылкам» описания эпика; раздел стал меньше — его предел в `sections` [docs-budgets.json](docs-budgets.json) понизить или убрать.
- **Как проверить:** `just -f spec/justfile docs-check-repo` — зелёный, а с ключом `"citations": true`, включённым локально, — без находок в этих разделах; `/code-review high`, один круг.
- **Разделы:** R171 spec, R160 spec; описание эпика; тело T184 в [docs/plan/E21.md](docs/plan/E21.md).

#### T186. Ссылки: architecture 3.3–4

- **Цель:** разделы 3.3–3.12 и 4 [docs/architecture.md](docs/architecture.md) ссылаются по R171 spec.
- **Сделать:** по «Ссылкам» описания эпика; раздел стал меньше — его предел в `sections` [docs-budgets.json](docs-budgets.json) понизить или убрать.
- **Как проверить:** как у T185, на этих разделах.
- **Разделы:** R171 spec, R160 spec; описание эпика.

#### T187. Ссылки: документы листов и sources

- **Цель:** [docs/sheets.md](docs/sheets.md), [docs/workbook.md](docs/workbook.md), [docs/essays.md](docs/essays.md), [docs/free-sheets.md](docs/free-sheets.md), [docs/near-sheets.md](docs/near-sheets.md) и [docs/sources.md](docs/sources.md) ссылаются по R171 spec.
- **Сделать:** по «Ссылкам» описания эпика; в `docs/sources.md` запись хранит источник, дату и цитату, без номера задачи, и в заголовке тоже; ссылки на запись из других документов сохраняют её номер — номера записей не меняются.
- **Как проверить:** как у T185, на этих файлах.
- **Разделы:** R171 spec, R143 spec; описание эпика.

#### T188. Ссылки: measurements 1–2

- **Цель:** разделы 1 и 2 [docs/measurements.md](docs/measurements.md) ссылаются по R171 spec.
- **Сделать:** по «Ссылкам» описания эпика; запись замера хранит что, как, на чём и числа, дату — в заголовке, без номера задачи; номера разделов не меняются.
- **Как проверить:** как у T185, на этих разделах.
- **Разделы:** R171 spec; описание эпика.

#### T189. Ссылки: measurements 3.1–3.10

- **Цель:** разделы 3.1–3.10 [docs/measurements.md](docs/measurements.md) ссылаются по R171 spec.
- **Сделать:** как T188.
- **Как проверить:** как у T185, на этих разделах.
- **Разделы:** R171 spec; описание эпика.

#### T190. Ссылки: measurements 3.11–5

- **Цель:** разделы [docs/measurements.md](docs/measurements.md) от 3.11 до конца, 4 и 5 тоже, ссылаются по R171 spec.
- **Сделать:** как T188; строки таблицы measurements 4.1 по эпикам — номера эпиков, не задач.
- **Как проверить:** как у T185, на этих разделах.
- **Разделы:** R171 spec; описание эпика.

#### T191. Журнал по R171 spec: начало и записи до «Checking line by line»

- **Цель:** начало [docs/decisions.md](docs/decisions.md) — по заготовке `spec/repo-template/files/docs/decisions.md`: хвост «— *Consequence:*» и запись без датированных поправок (R171 spec); записи разделов `##` от «The `spec` submodule» до «Checking line by line» — по R171 spec, их поправки свёрнуты.
- **Сделать:** по «Ссылкам» описания эпика; в начале журнала — его отличия от заготовки, которые держат его решения, остаются; запись стала меньше — её предел в `entries` [docs-budgets.json](docs-budgets.json) понизить или убрать.
- **Как проверить:** как у T185, на этих разделах; в описании pull request — таблица «поправка → место в новом тексте».
- **Разделы:** R171 spec, R160 spec; заготовка `docs/decisions.md`; описание эпика.

#### T192. Журнал по R171 spec: от «The check board» до «Move to `spec` `v0.2.43`»

- **Цель:** записи разделов `##` [docs/decisions.md](docs/decisions.md) от «The check board» до «Move to `spec` `v0.2.43`» — по R171 spec, их поправки свёрнуты.
- **Сделать:** как T191, без начала журнала.
- **Как проверить:** как у T191.
- **Разделы:** R171 spec, R160 spec; описание эпика.

#### T193. Журнал по R171 spec: от «Reading into an observation» до «OpenSSF Best Practices»

- **Цель:** записи разделов `##` [docs/decisions.md](docs/decisions.md) от «Reading into an observation» до «OpenSSF Best Practices» — по R171 spec, их поправки свёрнуты.
- **Сделать:** как T192.
- **Как проверить:** как у T191.
- **Разделы:** R171 spec, R160 spec; описание эпика.

#### T194. Журнал по R171 spec: от «The return of epic E3» до «The admission of words»

- **Цель:** записи разделов `##` [docs/decisions.md](docs/decisions.md) от «The return of epic E3» до «The admission of words» — по R171 spec, их поправки свёрнуты.
- **Сделать:** как T192.
- **Как проверить:** как у T191.
- **Разделы:** R171 spec, R160 spec; описание эпика.

#### T195. Журнал по R171 spec: от «Move to `spec` `v0.14.0`» до конца; ключ `citations`

- **Цель:** записи разделов `##` [docs/decisions.md](docs/decisions.md) от «Move to `spec` `v0.14.0`» до конца — по R171 spec, их поправки свёрнуты; `"citations": true` в [docs-budgets.json](docs-budgets.json), как в `spec/repo-template/files/docs-budgets.json`, и `docs-check-repo` с ним — зелёный во всём репозитории.
- **Сделать:** как T192; затем ключ; находки, которые остались в файлах прежних частей, — исправить здесь же.
- **Как проверить:** `just -f spec/justfile docs-check-repo` — зелёный с ключом; `/code-review high`, один круг; в «Сделано» — ссылок было и стало по всему репозиторию, по видам, командой `cite.mjs` measurements 1.18.4 spec, против замера «до» в «Сделано» T184.
- **Разделы:** R171 spec, R160 spec, R32 spec; описание эпика; строка 7.44 COORDINATION.md spec.
