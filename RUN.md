# RUN.md — work plan of tutor

What tutor does, does not do and has still to decide — [docs/architecture.md](docs/architecture.md); why — [docs/decisions.md](docs/decisions.md). The platform is in `spec/`, a submodule at tag v0.18.3: the component's card — architecture 3.6 spec; the order of repositories — repositories 6 spec; learners' data and law — privacy spec; the platform's decisions — spec's log. Small tasks, one at a time, gathered in epics; only the current epic is in detail, or two when a second runs beside it (R04; R19 spec). Other tables, each a file opening with its rules (R153 spec): 2, epics for later — [run/EPICS.md](run/EPICS.md); 3, questions to the author — [run/QUESTIONS.md](run/QUESTIONS.md); 4, debt — [run/DEBT.md](run/DEBT.md). No `run/TASKS.md`: a task outside an epic stands in table 1 (R103, R116).

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

## Table 1. Current epic — E10. Скорость CI: строка 7.16

**2026-11-16: эпик открыт (R118).** Задача T148 пришла от оркестратора `spec`, T171 spec, по строке 7.16 его COORDINATION.md; слово автора 2026-11-16 — ускорение CI сейчас. Текущего эпика не было — E9 сделан 2026-11-16 (R117), таблица 2 пуста, — а задача вне эпика по R103 — первая задача следующего, поэтому у строки 7.16 свой эпик, как у перехода — E9 (R115). Тела задач — в разделе «Epic E10» ниже.

**Результат:** CI `tutor` быстрее по строке 7.16 COORDINATION.md `spec`, а что проверки доказывают, не меняется: «Go / test» — без `-race` на пакетах, что не запускают горутин; кеш Go в CI и в job «Canvas» — по релизу `.github` строки 7.13; `.devcontainer/` — по тегу `spec` строки 7.14, со стадией `toolchain-core`.

**Основа:** R76 spec и его шестая поправка 2026-11-15, R156 spec; строки 7.13, 7.14 и 7.16 COORDINATION.md `spec`; measurements 3.1 spec; R92, R103, R117.

| Статус | ID | Задача | Кто | План | Зависит от |
|---|---|---|---|---|---|
| [x] | T148 | Скорость CI: строка 7.16 | Claude | — | — |
| [ ] | T149 | CI на релизе `.github` строки 7.13: кеш Go | Claude | — | T148; релиз `.github` строки 7.13 |
| [ ] | T150 | Переход на тег `spec` строки 7.14: стадия `toolchain-core` | Claude | — | T149; тег `spec` строки 7.14 |
| [x] | T151 | Срок возврата доступа своего ключа: строка 5.13, Д15 | Claude | — | — |

**Порядок:** T148 — сразу: `-race` ничего не ждёт. T149 — когда выйдет релиз `.github` строки 7.13; оркестратор запустит её тогда. T150 — последней: переход на тег меняет `.devcontainer/`, а пересобирает контейнер автор.

**Не входит:** Д6, Д11 и Д14; строка 5.12 COORDINATION.md `spec`.

**Вне эпика:** T151 — Д15, строка 5.13 COORDINATION.md `spec`: слово автора 2026-11-16 на ночь оркестратора (T171 `spec`) — строка таблицы 4, что закрывается внутри репозитория; по R103 она стоит в таблице 1 текущего эпика (R119).

## Epic E10. Скорость CI: строка 7.16

Открыт 2026-11-16 (R118); результат, основа и порядок — у таблицы 1. Тела T148 и T151 и их фразы итога — в архиве эпика, [docs/plan/E10.md](docs/plan/E10.md).

#### T149. CI на релизе `.github` строки 7.13: кеш Go

- **Цель:** CI `tutor` — на релизе `.github` строки 7.13 COORDINATION.md `spec`: кеш Go в job «Go» и тот же том `GOCACHE` в job «Canvas»; что проверки доказывают, не меняется.
- **Почему:** строка 7.16 COORDINATION.md `spec`, «Зависит от: 7.13 — для кеша»; measurements 3.1 spec — время уходит в компиляцию с пустым `GOCACHE`. T148 сделала часть, что не ждёт (R118).
- **Зависит от:** релиз `.github` строки 7.13 — `gh release list -R altnovatrail/.github`. Пока его нет, задача не начинается.
- **Сделать:**
  1. Что изменилось в общих workflow с v0.1.24: входы, права, имена проверок, на которые смотрит защита ветки, том кеша Go — `GOMODCACHE`, `GOCACHE`, `GOLANGCI_LINT_CACHE`, — файл и тег образа `toolchain`, CodeQL (go) без образа.
  2. Все `uses:` общих workflow в `.github/workflows/` — на SHA релиза с версией рядом.
  3. Свои job «Samples» и «Canvas» — образ `toolchain` тем же файлом и тегом, что общие; «Canvas» — тот же том `GOCACHE`, что job `go.yml`: входом релиза, если он есть, иначе копией его шагов, как в R117.
  4. Если релиз требует стадию `toolchain-core` в `.devcontainer/Dockerfile`, она приходит с T150: эта задача `.devcontainer/` не меняет.
  5. Правило `ci`, architecture 3.2, решение журнала, Д16 — что осталось.
- **Как проверить:** `actionlint` по сумме `self-ci.yml` `.github`; рецепты `ci-` в чистом клоне; CI pull request зелёный, имена проверок прежние; время изменённых job до и после — по прогонам `main` и pull request; `/code-review high`, один круг.
- **Разделы:** строки 7.13 и 7.16 COORDINATION.md `spec`; measurements 3.1 spec; R36 .github; R117, R118.

#### T150. Переход на тег `spec` строки 7.14: стадия `toolchain-core`

- **Цель:** `tutor` — на теге `spec` строки 7.14, со стадией `toolchain-core` в `.devcontainer/Dockerfile` по заготовке тега; проверки без браузера и Go-тестов, что релиз `.github` строки 7.13 гоняет в `toolchain-core`, идут в ней.
- **Почему:** строки 7.13 и 7.14 COORDINATION.md `spec`: «с тегом `spec` строки 7.14 — релиз и проверки без браузера и Go-тестов (lint, vuln, licenses, документы) в стадии `toolchain-core`»; переход на тег — задача своя (RULES.md spec, Across repositories).
- **Зависит от:** T149; тег `spec` строки 7.14 (T166 spec).
- **Сделать:**
  1. Переход на тег, как T145 (R115): подмодуль, модули Go `spec` и `spec/contracts`, `module-kit`, если он вышел на тот же тег; что тег принёс `tutor` — по `git -C spec log` и `grep -n` по `tutor` в `spec/run/DEBT.md`.
  2. `.devcontainer/` — по `just new-repo-draft tutor go 3.6 <tag>` в клоне `spec` во временной папке, с отличиями `tutor` (R12, R13, R69); apt — только источники снимка.
  3. CI — вход или версия `.github`, что берёт `toolchain-core`, если T149 её не взяла.
  4. Решение журнала; Д16 закрыть.
- **Мой шаг:** пересобрать контейнер после мержа.
- **Как проверить:** `just -f spec/justfile docs-check-repo`; рецепты `ci-` в чистом клоне; CI pull request зелёный; время изменённых job до и после; `/code-review high`, один круг.
- **Разделы:** строки 7.13, 7.14 и 7.16 COORDINATION.md `spec`; R31 spec; R115, R117, R118.
