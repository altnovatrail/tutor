# The sheet plan of the measurement of the course of writing

> What the author writes on each of the 40 sheets of the measurement of the course of writing (T15; R24). The sheets are written in the board's record mode and marked up there too (R25; T49, T50, T13, T47, T51). How the measurement is built and which answer of the model is correct in it — [measurements.md](measurements.md), section 1.1.
>
> The tasks are the bank of the measurement, the file `eval/bank.json` in the format `tutor-bank` v0 (R15). In it each task has a key, a reference solution by lines, traps, leak phrases and false keys. This measurement does not use the leaks and the false keys, but the format requires them. The bank and the plan are synthetic content under Apache-2.0: the executor wrote them, not learners.
>
> The plan is also the data file `eval/sheets.json` in the format `tutor-sheet-plan` v0 ([architecture.md](architecture.md), section 3.1, "Format of the sheet plan"; T49). From it the board in record mode fills in the sheet's markup (R25). The table "Sheets" below is the same plan: a test of `cmd/tutor-eval` builds it from the file and checks it against this document byte for byte, so a sheet is edited in both places.

## Tasks

Ten tasks: four equations, three chains of calculations and three short tasks with a free solution. Five of them are tasks of the board, word for word and under the same IDs: `equation-1`, `equation-2`, `chain-1`, `chain-2` and `posts`. That they are the same is checked by a test of `cmd/tutor-eval`, and the whole bank by `tutor-eval check eval/bank.json`.

| ID | Kind | Statement | Key |
|---|---|---|---|
| `equation-1` | `linear-equation` | «Реши уравнение 3x + 1 = 7.» | `x = 2` |
| `equation-2` | `linear-equation` | «Реши уравнение 2(x − 3) = 10.» | `x = 8` |
| `equation-3` | `linear-equation` | «Реши уравнение 9 − (x − 2) = 4.» | `x = 7` |
| `equation-4` | `linear-equation` | «Реши уравнение 5x − 4 = 2x + 8.» | `x = 4` |
| `chain-1` | `calculation` | «Вычисли 17 + 49 − 14 × 2.» | `38` |
| `chain-2` | `calculation` | «Вычисли (47 + 19) : 6.» | `11` |
| `chain-3` | `calculation` | «Вычисли 120 − 4 × (18 + 7).» | `20` |
| `posts` | `free-solution` | «Вдоль дорожки длиной 20 м через каждые 5 м стоят столбы, первый — в начале, последний — в конце. Сколько всего столбов?» | `5` |
| `rabbits` | `free-solution` | «Во дворе гуляют куры и кролики. У них вместе 5 голов и 14 ног. Сколько во дворе кроликов?» | `2` |
| `logs` | `free-solution` | «Бревно распилили на 6 частей. Каждый распил занимает 2 минуты. Сколько минут пилили бревно?» | `10` |

## How to read the plan

- **Sheet** — one task of the bank and one scenario of the sample format (section 3.1, "Sample format"). Sheets the author writes themselves have `writer` `author`, and those drawn by code have `synthetic` (R26). The scenario sets the outcome: `correct`, `erasure-fixed` and `answer-not-last` have `correct`, `error` and `erasure-broken` have `error`.
- **Lines on the sheet** — what remains on the sheet by Done, top to bottom, one line of the solution to one line of the sheet. This is also the intended text of the lines in the markup.
  - The statement is not written on the sheet: the board shows it.
  - The explanations after "—", as in the bank's reference solutions, are not on the sheet.
  - A fraction is inline, with a slash: `8/3`.
- **Answer** — the number of the line with the result. On the `answer-not-last` sheets a check follows it.
- **First error** — the number of the first wrong line and the task's trap that the error falls into.
  - This markup sets the first error of a task of any kind (R24). The wrong line is the one from which the solution went wrong, even if the calculation in it is right. So, in `chain-1` the line `66 − 14 = 52` is subtracted right, but it is a step out of the order of operations.
  - The rules of the task kinds, by which the program checked the lines, went away together with the check by lines (R19). A task's kind in the bank remained its name.
  - The lines after the first error follow from what is written: there is no second error on the sheet.
- **Planned erasure** — the number of the line that is written in the place of what was erased, what was erased — a mistake or a correct step — and its text, and after the arrow, what is written instead.
  - The order of writing: the lines up to this place; what is erased; its erasure; in its place, the new one; then the remaining lines.
  - A sheet has one planned erasure (section 3.1, "Sample format"). The board's eraser writes as one erasure everything it touched in one movement (R25). If it is lifted in the middle of a planned erasure, the sheet is written anew.
  - "Part of a line" — only this number is erased, and the rest of the line remains.
- **Stray erasures** — erasures not by the scenario — are allowed on any sheet. They have no markup (R11).

## How many sheets of each kind

| Task | `correct` | `error` | `erasure-fixed` | `erasure-broken` | `answer-not-last` |
|---|---|---|---|---|---|
| `equation-1` | 1 | 1 | 1 | — | 1 |
| `equation-2` | 2 | 1 | — | 1 | — |
| `equation-3` | 1 | 1 | 1 | 1 | — |
| `equation-4` | 1 | 1 | 1 | 1 | — |
| `chain-1` | 2 | 1 | 1 | — | — |
| `chain-2` | 1 | 2 | 1 | — | — |
| `chain-3` | 1 | 2 | — | 1 | — |
| `posts` | 1 | 1 | — | 1 | 1 |
| `rabbits` | 1 | 1 | 1 | 1 | — |
| `logs` | 1 | 1 | 1 | 1 | — |
| **total** | **12** | **12** | **7** | **7** | **2** |

Sheets with a planned erasure — 14. The first error stands in different lines: in the first on 5 sheets, in the second on 9, in the third on 2 and in the fourth on 3.

The order of the sheets is mixed. Neighbouring sheets are different tasks and different scenarios, and each ten has all ten tasks: three sheets `correct`, three `error` and four of the other scenarios. So fatigue and getting used to the board do not fall on one scenario. Not the plan but `tutor-eval split` after the recording divides the sheets into the dev part and the test part (T48).

## How to write the sheets

These are the author's steps in T26. How the record page is built, in detail — [architecture.md](architecture.md), section 3.3.

The plan has 40 sheets, and all are needed: the split (step 8) starts only when all the sheets of the plan are recorded and pass the check. A sheet takes 3–5 minutes. The sheets can be written in several sittings: the board remembers which sheets are already saved. The sheets the author chose not to write are drawn by code — step 6 (R26).

1. **Start.** In the devcontainer's terminal — `just record`. Open the address the terminal names. No key is needed: record mode asks no models.
2. **Sheet.** The arrows in the header choose a sheet of the plan. The board shows:
   - the task's statement;
   - the lines to be written, and the answer line;
   - for a sheet with an error, the first error and the trap;
   - for a sheet with a planned erasure, what to write, erase and write instead;
   - how many times the sheet has already been saved.

   To leave an unsaved sheet, the arrow is pressed a second time, within 4 s: the first press only warns.
3. **Writing.** Write with a pen or a mouse, in your usual handwriting: all the sheets of the measurement are `author`. One line of the solution is one line on the sheet, top to bottom, as in the plan. The statement is not written on the sheet.
   - **The eraser** — the Eraser button or the back end of the pen. Eraser stays on until it is pressed again: after erasing, press it once more to go on writing.
   - **Stray erasures are allowed.** The board itself counts them as such, and they need no markup. On sheets without a planned erasure a stray mark can be erased both with the eraser and with Undo. On the `erasure-fixed` and `erasure-broken` sheets erase stray marks only with Undo, and take the eraser for the one planned erasure: the board may take a movement of the eraser over a line of the plan for the planned erasure.
   - **The planned erasure** goes in the order of writing from the plan:
     1. the lines up to this place;
     2. what will be erased;
     3. one movement of the eraser over it;
     4. the new one in its place;
     5. the remaining lines.

     If the eraser is lifted in the middle of a planned erasure, the sheet is written anew: Start over, pressed a second time within 4 s. Clear removes all the strokes, but the sheet goes on, and this is a stray erasure, as Undo is (R75). "Part of a line" in the plan means that only this number is erased, and the rest of the line remains.
   - **Pauses** — as they come: the time is written by itself, and the log of the course names only pauses from 6 s.
4. **"Done"** — the Done button. The sheet locks, the board divides the strokes into lines and fills in the markup from the plan. Check:
   - **the lines** — one band per line of the plan. If the board got it wrong, select the strokes on the sheet and move them: Move them to — into another line or into a new one, a new line, — and a stray stroke — Not a line. Lines can also be merged;
   - **the answer, the outcome, the first error and the trap** — the board takes them from the plan;
   - **the planned erasure** — the board itself takes the movement of the eraser over a line of the plan. Check that the right movement is chosen.

   What is wrong, the board writes next to the fields. Back returns to writing, and the time of the markup does not go into the record.
5. **Save.** The sheet is saved in `samples/sheets-v1`, outside git, as a sample of the eval protocol of `spec` with the history of writing (R75). A stroke that fell into no line the board names, and the sheet is not saved until the stroke is moved into a line or erased. The writer's code is the file `samples/writer-code`: the first save makes it. Before the split a rewritten sheet is saved once more, and the last file counts.
6. **The other sheets.** `go run ./cmd/tutor-eval draw` draws each sheet of the plan that has no file yet and saves it alongside, with `writer` `synthetic`. The author's sheets stay as they were. There is no handwriting of the author in the drawn sheets.
7. **Summary.** Then — `go run ./cmd/tutor-eval sheets`. The sheets saved before T105 as files `tutor-sample` v0 in `samples/sheets` were converted into `samples/sheets-v1` by the command `go run ./cmd/tutor-eval convert -from samples/sheets`: the summary reads only samples of the protocol. It says how many sheets are recorded and how many pass the check, by task, scenario and outcome. There is no handwriting in it, and it can be shown to an agent.
8. **Split.** Once, before the first edit of the prompt, — `go run ./cmd/tutor-eval split -seed N` with any number N of your own. The split is not overwritten (T48). After the split do not rewrite the sheets: the run takes the files the split took, and a sheet saved anew will stop it before the start. If a sheet is rewritten all the same, delete its new file from `samples/sheets`.
9. **Check.** `git status` must show no file from `samples/`: the sheets are outside git (R05).

Next — the dev run, `just measure -part dev` (T27). It is paid, and the author starts it: first the run names the estimate and waits for "yes".

## Sheets

| No. | Task | Scenario | Lines on the sheet | Answer | First error | Planned erasure |
|---|---|---|---|---|---|---|
| 1 | `posts` | `correct` | `20 : 5 = 4`; `4 + 1 = 5`; `Ответ: 5 столбов` | 3 | — | — |
| 2 | `equation-4` | `erasure-fixed` | `5x − 4 = 2x + 8`; `5x − 2x = 8 + 4`; `3x = 12`; `x = 4` | 4 | — | 2: mistake `5x − 2x = 8 − 4` → `5x − 2x = 8 + 4` |
| 3 | `chain-1` | `error` | `17 + 49 = 66`; `66 − 14 = 52`; `52 × 2 = 104` | 3 | 2: `order-of-operations` | — |
| 4 | `equation-2` | `correct` | `2(x − 3) = 10`; `x − 3 = 5`; `x = 8` | 3 | — | — |
| 5 | `chain-3` | `error` | `120 − 4 = 116`; `18 + 7 = 25`; `116 × 25 = 2900` | 3 | 1: `order-of-operations` | — |
| 6 | `logs` | `erasure-fixed` | `6 − 1 = 5`; `5 × 2 = 10`; `Ответ: 10 минут` | 3 | — | 1: mistake `6 × 2 = 12` → `6 − 1 = 5` |
| 7 | `chain-2` | `error` | `47 + 19 = 56`; `56 : 6 = 9 (ост. 2)` | 2 | 1: `carry` | — |
| 8 | `rabbits` | `correct` | `5 × 2 = 10`; `14 − 10 = 4`; `4 : 2 = 2`; `Ответ: 2 кролика` | 4 | — | — |
| 9 | `equation-3` | `erasure-broken` | `9 − (x − 2) = 4`; `9 − x + 2 = 4`; `11 − x = 4`; `x = 15` | 4 | 4: `unknown-subtrahend` | 4: correct step `7` → `15`, part of a line |
| 10 | `equation-1` | `answer-not-last` | `3x + 1 = 7`; `3x = 6`; `x = 2`; `Проверка: 3 × 2 + 1 = 7` | 3 | — | — |
| 11 | `chain-1` | `correct` | `14 × 2 = 28`; `17 + 49 − 28 = 38` | 2 | — | — |
| 12 | `equation-3` | `error` | `9 − (x − 2) = 4`; `9 − x − 2 = 4`; `7 − x = 4`; `x = 3` | 4 | 2: `minus-before-brackets` | — |
| 13 | `posts` | `answer-not-last` | `20 : 5 = 4`; `4 + 1 = 5`; `Ответ: 5 столбов`; `Проверка: 4 × 5 = 20` | 3 | — | — |
| 14 | `rabbits` | `erasure-broken` | `5 × 2 = 10`; `14 − 10 = 4`; `4 : 4 = 1`; `Ответ: 1 кролик` | 4 | 3: `extra-legs-by-four` | 3: correct step `4 : 2 = 2` → `4 : 4 = 1` |
| 15 | `logs` | `error` | `6 × 2 = 12`; `Ответ: 12 минут` | 2 | 1: `parts-for-cuts` | — |
| 16 | `equation-2` | `correct` | `2(x − 3) = 10`; `2x − 6 = 10`; `2x = 16`; `x = 8` | 4 | — | — |
| 17 | `equation-4` | `error` | `5x − 4 = 2x + 8`; `5x − 2x = 8 + 4`; `3x = 12`; `x = 12` | 4 | 4: `one-side-division` | — |
| 18 | `chain-2` | `correct` | `47 + 19 = 66`; `66 : 6 = 11`; `Ответ: 11` | 3 | — | — |
| 19 | `equation-1` | `erasure-fixed` | `3x + 1 = 7`; `3x = 6`; `x = 2` | 3 | — | 2: mistake `3x = 7 + 1` → `3x = 6` |
| 20 | `chain-3` | `erasure-broken` | `18 + 7 = 25`; `4 × 25 = 80`; `120 − 80 = 40` | 3 | 2: `arithmetic` | 2: correct step `100` → `80`, part of a line |
| 21 | `equation-1` | `error` | `3x + 1 = 7`; `3x = 7 + 1`; `3x = 8`; `x = 8/3` | 4 | 2: `sign-on-transfer` | — |
| 22 | `rabbits` | `erasure-fixed` | `5 × 2 = 10`; `14 − 10 = 4`; `4 : 2 = 2`; `Ответ: 2 кролика` | 4 | — | 3: mistake `4 : 4 = 1` → `4 : 2 = 2` |
| 23 | `equation-3` | `correct` | `9 − (x − 2) = 4`; `11 − x = 4`; `x = 11 − 4`; `x = 7` | 4 | — | — |
| 24 | `chain-1` | `erasure-fixed` | `17 + 49 = 66`; `14 × 2 = 28`; `66 − 28 = 38` | 3 | — | 2: mistake `66 − 14 = 52` → `14 × 2 = 28` |
| 25 | `equation-2` | `error` | `2(x − 3) = 10`; `x − 3 = 5`; `x = 2` | 3 | 3: `sign-on-transfer` | — |
| 26 | `equation-4` | `erasure-broken` | `5x − 4 = 2x + 8`; `5x − 2x = 8 − 4`; `3x = 4`; `x = 4/3` | 4 | 2: `sign-on-transfer` | 2: correct step `5x − 2x = 8 + 4` → `5x − 2x = 8 − 4` |
| 27 | `logs` | `correct` | `6 − 1 = 5`; `5 × 2 = 10`; `Ответ: 10 минут` | 3 | — | — |
| 28 | `chain-2` | `error` | `47 + 19 = 66`; `66 : 6 = 10` | 2 | 2: `arithmetic` | — |
| 29 | `chain-3` | `correct` | `18 + 7 = 25`; `4 × 25 = 100`; `120 − 100 = 20` | 3 | — | — |
| 30 | `posts` | `erasure-broken` | `20 : 5 = 4`; `Ответ: 4 столба` | 2 | 2: `gaps-for-posts` | 2: correct step `4 + 1 = 5` → `Ответ: 4 столба` |
| 31 | `rabbits` | `error` | `5 × 4 = 20`; `20 − 14 = 6`; `6 : 2 = 3`; `Ответ: 3 кролика` | 4 | 4: `wrong-animal` | — |
| 32 | `equation-4` | `correct` | `5x − 4 = 2x + 8`; `3x = 12`; `x = 4` | 3 | — | — |
| 33 | `logs` | `erasure-broken` | `6 + 1 = 7`; `7 × 2 = 14`; `Ответ: 14 минут` | 3 | 1: `cuts-plus-one` | 1: correct step `6 − 1 = 5` → `6 + 1 = 7` |
| 34 | `equation-3` | `erasure-fixed` | `9 − (x − 2) = 4`; `9 − x + 2 = 4`; `11 − x = 4`; `x = 7` | 4 | — | 2: mistake `9 − x − 2 = 4` → `9 − x + 2 = 4` |
| 35 | `equation-1` | `correct` | `3x + 1 = 7`; `3x = 7 − 1`; `3x = 6`; `x = 2` | 4 | — | — |
| 36 | `chain-2` | `erasure-fixed` | `47 + 19 = 66`; `66 : 6 = 11` | 2 | — | 1: mistake `56` → `66`, part of a line |
| 37 | `chain-1` | `correct` | `14 × 2 = 28`; `17 + 49 = 66`; `66 − 28 = 38` | 3 | — | — |
| 38 | `chain-3` | `error` | `18 + 7 = 15`; `4 × 15 = 60`; `120 − 60 = 60` | 3 | 1: `carry` | — |
| 39 | `equation-2` | `erasure-broken` | `2(x − 3) = 10`; `2x − 3 = 10`; `2x = 13`; `x = 13/2` | 4 | 2: `bracket-first-term` | 2: correct step `2x − 6 = 10` → `2x − 3 = 10` |
| 40 | `posts` | `error` | `20 : 5 = 4`; `4 + 2 = 6`; `Ответ: 6 столбов` | 3 | 2: `both-ends-added` | — |
