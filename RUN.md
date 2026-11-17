# RUN.md — work plan of tutor

What tutor does, does not do and has still to decide — [docs/architecture.md](docs/architecture.md); why — [docs/decisions.md](docs/decisions.md). The platform is in `spec/`, a submodule at tag v0.18.6: the component's card — architecture 3.6 spec; the order of repositories — repositories 6 spec; learners' data and law — privacy spec; the platform's decisions — spec's log. Small tasks, one at a time, gathered in epics; only the current epic is in detail, or two when a second runs beside it (R04; R19 spec). Other tables, each a file opening with its rules (R153 spec): 2, epics for later — [run/EPICS.md](run/EPICS.md); 3, questions to the author — [run/QUESTIONS.md](run/QUESTIONS.md); 4, debt — [run/DEBT.md](run/DEBT.md). No `run/TASKS.md`: a task outside an epic stands in table 1 (R103, R116).

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
9. When done: table 1 `[ ]` → `[x]`; append to the body «Сделано (дата)» — what was done, how checked — and «Открыто». A question for the author from «Открыто» is also a row of [run/QUESTIONS.md](run/QUESTIONS.md); what is put off, a row of [run/DEBT.md](run/DEBT.md).
   - Then move the body verbatim, relative links fixed, to the epic archive `docs/plan/<epic>.md`, with a gist sentence — an unheaded list item at the start of the receiving section: «T01 — сделана <дата>: <итог одной фразой>». Neither stays here, and a search by heading finds one body, in the archive (R153 spec; the 2026-11-12 amendment to R23).
10. Language (R14 spec, R42 spec, R143 spec, R154 spec): this plan and `run/` in English; the epic's description, table 1, the bodies of tasks, the epic archives `docs/plan/` and the reports in Russian; `docs/` in English; README, CLAUDE.md, AGENTS.md, code, comments, schema descriptions, prompts and normative texts for authors in English.
11. A task larger than one review (≈ 30–60 minutes of reading): propose a split before starting.
12. Before a new task, close the findings on the previous one (R30 spec); an allowed departure from rule 1.
    - SonarCloud — [New Code](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor), on the task already merged; CodeQL and Scorecard — `gh api 'repos/altnovatrail/tutor/code-scanning/alerts?state=open'`.
    - Fix in code first; "false positive" is the last resort, with the reason in words: shared reasons — R33 spec, own ones — in the log. Never skip silently.
    - Report these fixes in a separate paragraph: the author commits them apart from the task.

**Statuses:** `[ ]` not started; `[x]` done; `[-]` not done, reason in the task; `[?]` deferred, reason and when to return in the task. A task that fails the author's review returns to `[ ]` with what to fix.

**Who:** «Claude» — the executor does it, the author reviews; «Claude, проверяю я» — texts the author reads line by line; «я + Claude» — the author does some steps: decisions, sign-ins, GitHub settings, confirmations by rule 4.

**Plan:** «да» — plan mode first: decisions missing from the documents, or larger than one review (rule 11); «—» — run at once.

**Task body** (Russian): «Цель», «Сделать», «Как проверить», «Разделы»; as needed «Не делать», «Решить при планировании», «Мой шаг», «Стоимость», «Риски»; from planning «Почему», «Решено автором (дата)», «Выяснилось при планировании»; when done «Сделано (дата)», «Открыто» (rule 9).

## Epics

- **Table 1** is the current epic, or two (R04): its tasks in detail, run one at a time.
- **No tasks outside an epic** (R103): a task that does not come from an epic — a move to a tag of `spec`, an analysis by R145 spec, an instruction of the author — stands in table 1 of the current epic; with none, it is the first task of the next, as T145 started E9 (R115).
- **Next epic.** Once table 1 is all checked, the author picks the next epic from [run/EPICS.md](run/EPICS.md) and splits it with Claude in plan mode into tasks of one review each, into table 1 and this file.
- **A done epic** goes to `docs/plan/<epic>.md` (R59 canvas-api): bodies and gist sentences are there already (rule 9); move its description and table of tasks verbatim; questions and debt stay in `run/`; until the next epic, table 1 reads «Current epic — none» and names the last done epic. Measure the epic's sessions — `just -f spec/justfile tokens-report --since <first day of the epic> --row` — as a row of measurements 4.1; a new analysis by the threshold of R145 spec.
- Never read an archive whole: find a task by heading — `grep -n '^#### T03\.' RUN.md docs/plan/*.md`; from another repository — «T03 tutor». Task numbers are global, never reused, and taken free at the moment of writing.

## Table 1. Current epic — E12. Записи допуска на моделях Anthropic

**2026-11-16: эпик открыт (R123).** Слово автора 2026-11-16 — «Открыть сейчас» — пришло от оркестратора `spec`, который прогоняет весь «Порядок» COORDINATION.md `spec` (R156 spec): строка 5.12 — эпик `tutor` (строка `api` В72; Д14). Эпик записала и спланировала T153 — её тело и фраза итога в архиве, [docs/plan/E12.md](docs/plan/E12.md); переход на тег `spec` уже сделан, T152 (`v0.18.6`, R122). Тела остальных задач — в разделе «Epic E12» ниже.

**Результат:** в `admission.Admitted` есть записи на моделях Anthropic для `CallWorkbook` и `CallEssay` — `handwriting_reading` на промптах тетради и чтения сочинения — и для `CallRubric` — `rubric_assessment` на промпте оценки, каждая со своим отпечатком промпта (R70 spec); ступень «author», для `adult`, на ИИ платформы и на своём ключе Anthropic, и отказ там, где развёртывание требует `full` (R80; поправки 2026-11-08 и 2026-11-15 к R76 spec); у каждого из трёх видов — ожидаемые токены для `Price`; релиз `tutor` с ними, и `api` открывает `ReadWorkbook`, `ReadEssay` и `AssessEssay` вместо `NotAdmitted` (строка `api` В72). Д14 закрыт.

**Основа:** строка 5.12 COORDINATION.md `spec`; R70, R71, R76, R80, R150, R156 spec; architecture 3.6 spec; R50, R62, R80, R97, R98, R99, R113; architecture 3.9, 3.10; measurements 3.7, 3.8; Д14.

| Статус | ID | Задача | Кто | План | Зависит от |
|---|---|---|---|---|---|
| [x] | T153 | Запись и план эпика E12 | Claude | — | — |
| [x] | T154 | Решения автора о допуске тетради, сочинения и рубрики | я + Claude | да | T153 |
| [x] | T155 | Правило допуска чтения тетради | Claude | — | T154 |
| [x] | T156 | Правило допуска чтения сочинения и оценки по рубрике | Claude | — | T154 |
| [-] | T157 | Листы автора: тетрадь и сочинения | я + Claude | да | T154 |
| [-] | T158 | Допуск чтения тетради: прогон и запись | я + Claude | — | T155 |
| [-] | T159 | Допуск сочинения и рубрики: прогоны и записи | я + Claude | — | T156, T158 |
| [ ] | T160 | Релиз для `api` и итог эпика | Claude | — | T163 |
| [x] | T161 | Режим чтения почерка: пара или один Haiku 5.5 · medium | Claude | — | — |
| [x] | T163 | Записи допуска тетради, сочинения и рубрики словом автора | Claude | — | — |

**Порядок:** T154 — первой: без решений автора правилам допуска нечего строить. T155, T156 и T157 — после неё, без трат, в любом порядке; T157 нужна, только если T154 решит допускать на почерке автора; задачу, которая по ответам не нужна, T154 ставит `[-]` и убирает из «Зависит от» следующих. T158 и T159 — платные: каждая сначала возвращает вопрос о потолке, а тратит только после ответа автора (правило 4); T159 после T158 — один репозиторий идёт по одной задаче, а `price.go` и `admission/models.go` правят обе. T160 — последней. У каждой задачи с кодом свой релиз (R62); релиз T160 — тот, что несёт все три записи. *2026-11-16 (R124): T154 сделана, T157 — `[-]`: набор допуска — синтетика кодом.* *2026-11-17 (R125): T161 — слово автора вне плана эпика, через оркестратор `spec` (R103); сделана, тело — в архиве.* *2026-11-17 (R126): T163 — слово автора вне плана эпика, через оркестратор `spec` (R103): eval перед допуском (R07 spec) приостановлен до 2027-01-17, и три записи стоят словом автора, без трат; T158 и T159 — `[-]`, T160 — после T163; тела трёх — в архиве; возврат к прогонам на Haiku — Д17.*

**Не входит:** ступень `full` — она ждёт образцов взрослых добровольцев (П-16; поправка 2026-11-15 к R76 spec); модели не Anthropic; вызовы в `api` — задача `api` после релиза; Д6 и Д11.

## Epic E12. Записи допуска на моделях Anthropic

Открыт 2026-11-16 (R123); результат, основа и порядок — у таблицы 1. Общее для задач с кодом, как в E5–E8: код публичный, eval мерит тот же код, что вызывает `api` (R70 spec), журналы без содержания, тесты — на заглушке и без ключа; запись списка называет отчёт, который её допустил, его sha256 и отпечаток промпта (R80). Решение автора, которого нет в документах, — строкой [run/QUESTIONS.md](run/QUESTIONS.md) и статусом `waiting` (правило 7, контракт исполнителя `spec`).

#### T160. Релиз для `api` и итог эпика

- **Цель:** релиз `tutor`, который несёт записи T163 — словом автора, без прогонов (R126; T158 и T159 — `[-]`), — проверен, и `api` знает его версию; эпик E12 — в архиве.
- **Почему:** строка 5.12 COORDINATION.md `spec`: «релиз»; R62.
- **Сделать:** `gh release verify` релиза после мержа T163 — `release.yml` выпускает его сам (R62); его версия — в architecture 3.9, README и решении журнала; Д14 закрыт, а ступень `full` трёх видов — новая строка [run/DEBT.md](run/DEBT.md): она ждёт образцов взрослых добровольцев (П-16), и без строки к ней ничто не вернёт; строка «В реестр spec» — версия, которой строка 5.12 сделана, для `spec` и для `api` (R81 spec); архив эпика по правилам Epics этого файла и строка сессий эпика в measurements 4.1.
- **Как проверить:** `gh release verify`; `just -f spec/justfile docs-check-repo`.
- **Разделы:** R62; строка 5.12 COORDINATION.md `spec`; строка `api` В72; Д14; RUN.md, Epics.
