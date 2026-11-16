# RUN.md — work plan of tutor

What tutor does, does not do and has still to decide — [docs/architecture.md](docs/architecture.md); why — [docs/decisions.md](docs/decisions.md). The platform is in `spec/`, a submodule at tag v0.18.3: the component's card — architecture 3.6 spec; the order of repositories — repositories 6 spec. Small tasks, one at a time, gathered in epics; only the current epic is in detail, or two when a second runs beside it (R04; R19 spec). Other tables, each a file opening with its rules (R153 spec): 2, epics for later — [run/EPICS.md](run/EPICS.md); 3, questions to the author — [run/QUESTIONS.md](run/QUESTIONS.md); 4, debt — [run/DEBT.md](run/DEBT.md). No `run/TASKS.md`: a task outside an epic stands in table 1 (R103, R116).

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

## Table 1. Current epic — E9. Переход на тег `spec` v0.18.3

**2026-11-16: эпик открыт (R115).** Слово автора «Сделай эпик E14» в оркестраторе `spec`: его волна 0 первым переводит каждый репозиторий эпика на последний тег `spec` (R156 spec). Текущего эпика не было — E8 сделан 2026-11-14 (R114), таблица 2 пуста, — а переход по R103 — первая задача следующего эпика, поэтому у перехода свой эпик, как E11 `sim` (R60 sim). Тела задач — в разделе «Epic E9» ниже.

**Результат:** `tutor` стоит на `spec` v0.18.3 — подмодуль, модули Go, контейнер и `.claude/` по заготовке тега, таблица 3 без строк, которые тег записал; план — в раскладке и на языке R153–R155 spec (Д250 spec); CI — на последнем релизе `.github` с кешем образа `toolchain` и группой Dependabot `shared-workflows` (Д146, Д88 spec).

**Основа:** R156 spec, R76 spec; RULES.md spec; repositories 2.2 spec; Д88, Д146, Д195, Д248, Д250 spec; R102, R103, R110.

| Статус | ID | Задача | Кто | План | Зависит от |
|---|---|---|---|---|---|
| [x] | T145 | Переход на тег `spec` v0.18.3 | Claude | — | — |
| [ ] | T146 | План по R153–R155 spec: таблицы в `run/`, план на английском | Claude | — | T145 |
| [ ] | T147 | CI на релизе `.github` v0.1.24 | Claude | — | T145 |

**Порядок:** T145 — первой, по тексту оркестратора. T146 и T147 друг от друга не зависят, но обе меняют план и `docs/decisions.md`, поэтому идут по одной: T146, затем T147.

**Не входит:** строки 5.12, 5.13 и 7.16 COORDINATION.md `spec` — долги Д14–Д16: это работа `tutor`, а не переход; Д6 и Д11.

## Epic E9. Переход на тег `spec` v0.18.3

Открыт 2026-11-16 (R115); результат, основа и порядок — у таблицы 1. Поведение кода эпик не меняет: модули Go `spec` поднимаются с правкой комментариев и теста таблицы допуска 0.4, а CI меняет только ссылки на общие workflow.

#### T146. План по R153–R155 spec: таблицы в `run/`, план на английском

- **Цель:** план `tutor` — в раскладке и на языке заготовки `spec` v0.18.3: таблицы 2, 3 и 4 — `run/EPICS.md`, `run/QUESTIONS.md`, `run/DEBT.md`; правила RUN.md и таблицы — по-английски, коротко, для сессии; тела задач и архивы эпиков — по-русски (Д250 spec).
- **Почему:** вторая часть перехода T145 по правилу 11 (R115): R153–R155 spec пришли с тегом v0.18.3, а `docs-check` и `just inbox` `spec` держат старую раскладку только до последнего переехавшего репозитория (Д250 spec).
- **Сделать:**
  1. По заготовке `just new-repo-draft tutor go 3.6 v0.18.3` на клоне `spec` v0.18.3 во временной папке: `run/EPICS.md` — таблица 2; `run/QUESTIONS.md` — таблица 3 со следующим свободным номером; `run/DEBT.md` — таблица 4 вместо `DEBT.md`, его строки по-английски, а абзацы закрытых долгов уходят из начала таблицы (Д250 spec); `docs-budgets.json` — пределы новых файлов, если нужны.
  2. RUN.md — правила «Как пользоваться» и «Как устроен план: эпики» по-английски, коротко, по заготовке, номера правил прежние, с отличиями, которые держат решения этого журнала: правило 4 — R110 (доска, eval, ключ), правило 12 — SonarCloud, CodeQL и Scorecard. R103 — задачи вне эпика нет, она встаёт в эпик, — против `run/TASKS.md` R155 spec: взвесить и записать; поправка 2026-11-13 к R19 spec оставляет репозиторию его правило, если оно ставит такую задачу в эпик. Таблица 1 и тела задач — по-русски.
  3. CLAUDE.md — раздел Documents по заготовке: RUN.md и `run/`; README.md — план по-английски; ссылки на `DEBT.md` в `docs/`, `.claude/rules/` и коде — на `run/DEBT.md`; [docs/plan/questions.md](docs/plan/questions.md) — ссылка из правил таблицы 3; архивы `docs/plan/` не переписываются.
  4. Решение журнала — раскладка и язык плана, что взято из заготовки и что нет; поправки к R19, R23, R103 и к записям, которые называют `DEBT.md`.
- **Как проверить:** `just -f spec/justfile docs-check-repo` — зелёный, таблицы 3 и 4 — в `run/`, без запасного чтения старой раскладки; `node` с `rowsForSpec` из `spec/tools/inbox/inbox.mjs` находит таблицу 3 в `run/QUESTIONS.md`; `/code-review high`, один круг.
- **Разделы:** R153, R154, R155 spec; Д250 spec; R19 spec и её поправка 2026-11-13; R19, R23, R103, R110, R115.

#### T147. CI на релизе `.github` v0.1.24

- **Цель:** все `uses:` общих workflow `.github` в `.github/workflows/` — на последнем релизе `.github`, v0.1.24, по SHA с версией рядом; образ `toolchain` CI берёт из кеша GitHub Actions по sha256 `.devcontainer/Dockerfile` (Д146 spec); ссылки на общие workflow Dependabot поднимает одним pull request (Д88 spec).
- **Почему:** шаг 6 текста T145 оркестратора, как T68 `sim`: `tutor` на v0.1.15, а `release.yml` — на v0.1.11 (Д146 spec), и каждый job строит образ из сети — сбой nodejs.org, GitHub или снимка Ubuntu роняет проверки.
- **Сделать:**
  1. `gh release view v0.1.24 -R altnovatrail/.github` и SHA тега; что изменилось в общих `ci`-, `go.yml`, `release.yml`, `codeql.yml`, `scorecard.yml`, `documents.yml`, `secrets.yml` и `dco.yml` с v0.1.11 и v0.1.15 — входы, права, имена проверок, на которые смотрит защита ветки.
  2. `uses:` всех вызовов — на SHA v0.1.24 с `# v0.1.24`; свои job «Samples» и «Canvas» — образ `toolchain` тем же путём кеша, что и общие, если релиз даёт для этого вход.
  3. `.github/dependabot.yml` — группа `shared-workflows`, `patterns: ["altnovatrail/.github/*"]`, под `github-actions` (Д88 spec).
  4. Правило `ci` — что CI берёт образ из кеша; решение журнала — переход CI и что взято.
- **Как проверить:** `actionlint` по сумме `self-ci.yml` `.github`; рецепты `ci-` в чистом клоне; CI pull request — зелёный, имена проверок прежние; `/code-review high`, один круг.
- **Разделы:** Д88, Д146 spec; R31 spec и её поправка 2026-11-09; R36 .github; R13, R30, R66.

