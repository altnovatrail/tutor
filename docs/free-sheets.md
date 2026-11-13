# The sheet plan of the measurement of free writing

> What code draws on each of the 40 sheets of the measurement of free writing, and what the author writes with a mouse on their three (T57, T62; R28). How the measurement is built and which answer of the judge is correct in it — [measurements.md](measurements.md), section 3.3.
>
> The tasks are the bank of free writing, the file `eval/free-bank.json` in the format `tutor-bank` v0 (R15; T52). The bank and the plans are synthetic content under Apache-2.0: the executor wrote them, not learners.
>
> The plans are data files in the format `tutor-sheet-plan` v1 ([architecture.md](architecture.md), section 3.1, "Format of the sheet plan", version 1):
> - `eval/free-sheets.json` — 40 sheets, drawn by code (T59, T60);
> - `eval/free-author.json` — the three sheets of the author, free solutions of three tasks, one sheet per layout (T62).
>
> The tables below are the same bank and plans: a test of `cmd/tutor-eval` builds them from the files and checks them against this document byte for byte, so a sheet is edited in both places.

## Tasks

The six tasks of the board with free writing, tasks 6–11, word for word and under the same IDs ([architecture.md](architecture.md), section 3.3). The task sets the sheet's layout: two have a number line, two a drawing, a row of houses or a scheme of segments, and in two the counting goes at the side, in a column. That the tasks of the bank are the same as on the board is checked by a test of `cmd/tutor-eval`, and the whole bank by `tutor-eval check eval/free-bank.json`.

| ID | Layout | Statement | Key |
|---|---|---|---|
| `grasshopper` | number line | «Кузнечик сидит на числовой прямой в точке 3. Он прыгнул три раза вправо, каждый раз на 4, а потом один раз влево на 5. В какой точке он оказался?» | `10` |
| `backwards` | number line | «Задумали число. К нему прибавили 7, потом вычли 12 и получили 15. Какое число задумали?» | `20` |
| `houses` | drawing: a row of objects | «В ряд стоят домики. Домик с флажком — четвёртый слева и шестой справа. Сколько домиков в ряду?» | `9` |
| `stamps` | drawing: a scheme of segments | «У двух братьев вместе 20 марок. У старшего на 6 марок больше, чем у младшего. Сколько марок у младшего?» | `7` |
| `pages` | working at the side | «Страницы книги пронумеровали подряд, с 1 по 75. Сколько всего цифр написали?» | `141` |
| `album` | working at the side | «Тетрадь стоит 14 рублей, а альбом — в 3 раза дороже. Сколько стоят вместе 2 альбома и 5 тетрадей?» | `154` |

## How to read the plan

- **Sheet** — one task of the bank and one scenario of the sample format ([architecture.md](architecture.md), section 3.1, "Sample format"). The sheets of code have `writer` `synthetic`, the author's sheets have `author` (R26). The scenario sets the outcome: `correct` and `erasure-fixed` have `correct`, `error` and `erasure-broken` have `error`.
- **Parts on the sheet** — what remains on the sheet by the end, in the order in which the solution is written and read. A part has a number, a kind and a place:
  - with no mark — a line of text in the main column, on the left;
  - "column at the side" — a calculation in a column to the right of the main column, as the author wrote in the pilot: the first number on top, under it the sign and the second number, a bar and the result. In the table the column is written inline;
  - "drawing" — a number line, a row of houses or a scheme of segments, in the main column or "full width", across the whole sheet. In the table, its description in words: this is the text of the part in the sample's markup. What is drawn is in the table "Drawings".

  The statement is not written on the sheet: the board shows it. The explanations after "—", as in the bank's reference solutions, are not on the sheet.
- **□ and △** — unknowns, as the author wrote them in the pilot:
  - on three `backwards` sheets, in the text: the statement is written as the equality `□ + 7 − 12 = 15`, and □ is the number thought of;
  - on three `stamps` sheets, the labels of the scheme's segments: □ is the segment of the younger brother, whose stamps the task asks about, △ that of the elder. On a scheme without labels it is not seen which segment is whose, so the trap `longer-segment` stands only on the sheets with □ and △.
- **Answer** — the number of the part with the result. It is always text and always the last part.
- **First error** — the number of the part and the task's trap that the error falls into.
  - The markup sets the first error (R24). The wrong part is the one from which the solution went wrong, even if the calculation in it is right: so, in the task about the grasshopper `15 + 5 = 20` is added right, but the jump is in the wrong direction.
  - The parts after it follow from what is written: there is no second error on the sheet. A drawing after a wrong line draws what is written in it.
  - The errors are the bank's traps, with no subtleties beyond them (the 2026-10-27 amendment to R28): a wrong arc ends two divisions or more from the correct one, not on the neighbouring one, and a wrong column has a wrong step, not one digit. There are no slips in the calculation: every equality on the sheets is right.
- **Planned erasure** — the number of the part written in the place of what was erased, what was erased — a mistake or a correct step, — and what was there, and after the arrow, what came instead:
  - in the text — the whole line, and with the mark "part of a line", one number of it, and the rest of the line remains;
  - "in a column" — one number of it. If the top or the bottom number is erased, the result is written only after the erasure;
  - "in the drawing" — its elements, and what was erased and what is new are named in words, by pieces of its description. What the drawing was like by the erasure is in the table "Drawings", the column "Before the erasure": the erasure removes what is not in the final drawing, and then the new is drawn in.

  The order of writing: the parts up to this place; what is erased; its erasure, in one movement of the eraser; the new in its place; then the remaining parts.
- **Stray erasures** — erasures not by the scenario — are allowed on any sheet. They have no markup (R11).

## How many sheets of each kind

The composition is the one T54 recorded ([measurements.md](measurements.md), section 3.3):

| Task | Layout | `correct` | `error` | `erasure-fixed` | `erasure-broken` | Total |
|---|---|---|---|---|---|---|
| `grasshopper` | number line | 2 | 2 | 2 | 1 | 7 |
| `backwards` | number line | 2 | 2 | 1 | 2 | 7 |
| `houses` | drawing: a row of objects | 2 | 2 | 2 | 1 | 7 |
| `stamps` | drawing: a scheme of segments | 2 | 2 | 1 | 1 | 6 |
| `pages` | working at the side | 2 | 2 | 1 | 1 | 6 |
| `album` | working at the side | 2 | 2 | 1 | 2 | 7 |
| **total** | | **12** | **12** | **8** | **8** | **40** |

Where the first error and the planned erasure stand, by layout. "In the drawing" — in the drawing, "in the text" — in a line of the main column, "at the side" — in the working at the side, to the right of it, and the working at the side on these sheets is always a column:

| Layout | Sheets | With a mistake | First error: in the drawing / in the text / at the side | Erasures | Erasure: in the drawing / in the text / at the side |
|---|---|---|---|---|---|
| number line | 14 | 7 | 4 / 3 / — | 6 | 3 / 3 / — |
| drawing | 13 | 6 | 3 / 3 / — | 5 | 3 / 2 / — |
| working at the side | 13 | 7 | — / 3 / 4 | 5 | — / 2 / 3 |
| **total** | **40** | **20** | **7 / 9 / 4** | **16** | **6 / 7 / 3** |

The planned erasure, and with it the frames, is on 16 sheets. The first error stands in different parts of the sheet: in the first on 9 sheets, in the second on 10 and in the third on 1, and in the answer on none.

The order of the sheets is mixed. Neighbouring sheets are different tasks and different scenarios, and each ten has three sheets `correct`, three `error`, two `erasure-fixed` and two `erasure-broken`. Not the plan but `tutor-eval split -seed 20261027` divides the sheets into the dev part and the test part — once, before the first edit of the prompt (T66).

## The author's sheets

Three sheets — free solutions of three tasks, one per layout of their sheets of code: the grasshopper on the number line, the stamps on the scheme, the album with the counting at the side (the 2026-10-27 amendment to R28). The author solves the task the way they would solve it themselves: these sheets have no scenario, no planned erasure and no deliberate error, and the whole sheet is one part. That the author's sheets are such is checked by a test of `cmd/tutor-eval`.

The author's sheets have no split and no threshold: their run is after the prompt is frozen, and the report is next to the synthetic sheets, by sheet (T77).

| No. | Task | Layout of the sheets of code | Scenario | Part on the sheet |
|---|---|---|---|---|
| 1 | `grasshopper` | number line | `correct` | full width: `свободное решение целиком` |
| 2 | `stamps` | drawing: a scheme of segments | `correct` | full width: `свободное решение целиком` |
| 3 | `album` | working at the side | `correct` | full width: `свободное решение целиком` |

## How to write the author's sheets

This is the author's step in T62: the recipe `just free-record`, the directory `samples/free`.
1. Choose a sheet with the arrows, solve the task with the mouse — in any way you like, — and press Done.
2. The board will outline the whole sheet with one box: this is one part, and the markup needs no editing. Press Save.
3. Made a mistake by accident — press Clear and solve it anew: the author's sheet is a correct solution. Erase stray marks as is convenient — with the eraser or Undo.

After Save — the summary `go run ./cmd/tutor-eval sheets -dir samples/free -bank eval/free-bank.json -plan eval/free-author.json`. There is no handwriting in it, and its output goes to the executor. `git status` must show no file from `samples/`: the sheets are outside git (R05).

## The sheets of code

The sheets are drawn by `tutor-eval draw -seed 1` into the directory `synthetic/free`, with `writer` `synthetic` (T59, T60).

| No. | Task | Scenario | Parts on the sheet | Answer | First error | Planned erasure |
|---|---|---|---|---|---|---|
| 1 | `grasshopper` | `correct` | 1) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10`; 2) `3 + 4 + 4 + 4 = 15`; 3) `15 − 5 = 10`; 4) `Ответ: в точке 10` | 4 | — | — |
| 2 | `album` | `error` | 1) column at the side: `14 × 3 = 42`; 2) column at the side: `42 × 5 = 210`; 3) column at the side: `14 × 2 = 28`; 4) `210 + 28 = 238`; 5) `Ответ: 238 рублей` | 5 | 2: `swapped-counts` | — |
| 3 | `houses` | `erasure-fixed` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 5 справа, под домиками номера от 1 до 9`; 2) `3 + 1 + 5 = 9`; 3) `Ответ: 9 домиков` | 3 | — | 1: mistake `4 слева` → `3 слева`, in the drawing |
| 4 | `backwards` | `erasure-broken` | 1) `15 + 12 = 27`; 2) drawing full width: `числовая прямая, с конца: прыжок вправо 15 → 27 и прыжок влево 27 → 22`; 3) `Ответ: 22` | 3 | 2: `jump-length` | 2: correct step `27 → 20` → `27 → 22`, in the drawing |
| 5 | `stamps` | `correct` | 1) drawing: `схема: отрезок младшего □ и отрезок старшего △ длиннее на 6, вместе 20`; 2) `20 − 6 = 14`; 3) `14 : 2 = 7`; 4) `Ответ: 7 марок` | 4 | — | — |
| 6 | `pages` | `error` | 1) `с 1 по 75: по 2 цифры`; 2) column at the side: `75 × 2 = 150`; 3) `Ответ: 150 цифр` | 3 | 1: `all-two-digit` | — |
| 7 | `grasshopper` | `erasure-fixed` | 1) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10`; 2) `3 + 4 + 4 + 4 = 15`; 3) `15 − 5 = 10`; 4) `Ответ: в точке 10` | 4 | — | 1: mistake `7 → 13` → `7 → 11`, in the drawing |
| 8 | `stamps` | `erasure-broken` | 1) drawing: `схема: отрезок младшего □ длиннее на 6 и отрезок старшего △, вместе 20`; 2) `(20 − 6) : 2 = 7`; 3) `7 + 6 = 13`; 4) `Ответ: 13 марок` | 4 | 1: `longer-segment` | 1: correct step `△ длиннее на 6` → `□ длиннее на 6`, in the drawing |
| 9 | `album` | `correct` | 1) column at the side: `14 × 3 = 42`; 2) column at the side: `42 × 2 = 84`; 3) column at the side: `14 × 5 = 70`; 4) `84 + 70 = 154`; 5) `Ответ: 154 рубля` | 5 | — | — |
| 10 | `houses` | `error` | 1) drawing full width: `ряд домиков — 4 слева от домика с флажком, он сам и 5 справа`; 2) `4 + 1 + 5 = 10`; 3) `Ответ: 10 домиков` | 3 | 1: `place-for-count` | — |
| 11 | `backwards` | `correct` | 1) `□ + 7 − 12 = 15`; 2) `□ = 15 + 12 − 7`; 3) drawing full width: `числовая прямая, с конца: прыжок вправо 15 → 27 и прыжок влево 27 → 20`; 4) `Ответ: 20` | 4 | — | — |
| 12 | `grasshopper` | `error` | 1) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 12`; 2) `Ответ: в точке 12` | 2 | 1: `jump-length` | — |
| 13 | `pages` | `erasure-broken` | 1) `с 1 по 9: 9 цифр`; 2) `с 10 по 75: 65 стр.`; 3) column at the side: `65 × 2 = 130`; 4) `9 + 130 = 139`; 5) `Ответ: 139 цифр` | 5 | 2: `count-from-to` | 2: correct step `66` → `65`, part of a line |
| 14 | `stamps` | `erasure-fixed` | 1) drawing: `схема: отрезок младшего □ и отрезок старшего △ длиннее на 6, вместе 20`; 2) `(20 − 6) : 2 = 7`; 3) `Ответ: 7 марок` | 3 | — | 1: mistake `□ длиннее на 6` → `△ длиннее на 6`, in the drawing |
| 15 | `album` | `correct` | 1) `14 × 3 = 42`; 2) column at the side: `42 × 2 = 84`; 3) column at the side: `14 × 5 = 70`; 4) column at the side: `84 + 70 = 154`; 5) `Ответ: 154 рубля` | 5 | — | — |
| 16 | `backwards` | `error` | 1) drawing full width: `числовая прямая, с конца: прыжок вправо 15 → 25 и прыжок влево 25 → 18`; 2) `25 − 7 = 18`; 3) `Ответ: 18` | 3 | 1: `jump-length` | — |
| 17 | `houses` | `erasure-broken` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 5 справа`; 2) `3 + 5 = 8`; 3) `Ответ: 8 домиков` | 3 | 2: `self-forgotten` | 2: correct step `3 + 1 + 5 = 9` → `3 + 5 = 8` |
| 18 | `pages` | `erasure-fixed` | 1) `с 1 по 9: 9 цифр`; 2) `с 10 по 75: по 2 цифры`; 3) column at the side: `75 − 9 = 66`; 4) column at the side: `66 × 2 = 132`; 5) `9 + 132 = 141`; 6) `Ответ: 141 цифра` | 6 | — | 3: mistake `10` → `9`, in a column |
| 19 | `houses` | `correct` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 5 справа, под домиками номера от 1 до 9`; 2) `Ответ: 9 домиков` | 2 | — | — |
| 20 | `stamps` | `error` | 1) drawing: `схема: отрезок младшего и отрезок старшего, длиннее на 6, — вместе 20`; 2) `20 : 2 = 10`; 3) `Ответ: 10 марок` | 3 | 2: `halved-total` | — |
| 21 | `album` | `erasure-broken` | 1) column at the side: `14 × 3 = 42`; 2) column at the side: `14 × 2 = 28`; 3) column at the side: `42 × 5 = 210`; 4) `28 + 210 = 238`; 5) `Ответ: 238 рублей` | 5 | 2: `swapped-counts` | 2: correct step `5` → `2`, in a column |
| 22 | `grasshopper` | `correct` | 1) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10`; 2) `Ответ: в точке 10` | 2 | — | — |
| 23 | `houses` | `error` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 6 справа, под домиками номера от 1 до 10`; 2) `Ответ: 10 домиков` | 2 | 1: `place-for-count` | — |
| 24 | `backwards` | `erasure-fixed` | 1) `□ + 7 − 12 = 15`; 2) drawing full width: `числовая прямая, с конца: прыжок вправо 15 → 27 и прыжок влево 27 → 20`; 3) `15 + 12 − 7 = 20`; 4) `Ответ: 20` | 4 | — | 3: mistake `15 − 12 + 7 = 10` → `15 + 12 − 7 = 20` |
| 25 | `pages` | `correct` | 1) `с 1 по 9: 9 цифр`; 2) `с 10 по 75: 66 стр.`; 3) column at the side: `66 × 2 = 132`; 4) column at the side: `132 + 9 = 141`; 5) `Ответ: 141 цифра` | 5 | — | — |
| 26 | `grasshopper` | `erasure-broken` | 1) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 13 и прыжок влево 13 → 8`; 2) `13 − 5 = 8`; 3) `Ответ: в точке 8` | 3 | 1: `jump-length` | 1: correct step `11 → 15` → `11 → 13`, in the drawing |
| 27 | `stamps` | `error` | 1) drawing: `схема: отрезок младшего и отрезок старшего, длиннее на 6, — вместе 20`; 2) `20 + 6 = 26`; 3) `26 : 2 = 13`; 4) `Ответ: 13 марок` | 4 | 2: `difference-added` | — |
| 28 | `houses` | `erasure-fixed` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 5 справа`; 2) `3 + 1 + 5 = 9`; 3) `Ответ: 9 домиков` | 3 | — | 2: mistake `4 + 6 = 10` → `3 + 1 + 5 = 9` |
| 29 | `backwards` | `correct` | 1) drawing full width: `числовая прямая, с конца: прыжок вправо 15 → 27 и прыжок влево 27 → 20`; 2) `15 + 12 = 27`; 3) `27 − 7 = 20`; 4) `Ответ: 20` | 4 | — | — |
| 30 | `pages` | `error` | 1) `с 1 по 9: 9 цифр`; 2) `с 10 по 75: по 2 цифры`; 3) column at the side: `75 − 10 = 65`; 4) column at the side: `65 × 2 = 130`; 5) `9 + 130 = 139`; 6) `Ответ: 139 цифр` | 6 | 3: `count-from-to` | — |
| 31 | `album` | `erasure-fixed` | 1) `14 × 3 = 42`; 2) column at the side: `42 × 2 = 84`; 3) column at the side: `14 × 5 = 70`; 4) `84 + 70 = 154`; 5) `Ответ: 154 рубля` | 5 | — | 3: mistake `2` → `5`, in a column |
| 32 | `backwards` | `error` | 1) `□ + 7 − 12 = 15`; 2) `15 − 12 + 7 = 10`; 3) drawing full width: `числовая прямая, с конца: прыжок влево 15 → 3 и прыжок вправо 3 → 10`; 4) `Ответ: 10` | 4 | 2: `not-inverted` | — |
| 33 | `houses` | `correct` | 1) drawing full width: `ряд домиков — 3 слева от домика с флажком, он сам и 5 справа`; 2) `4 + 6 − 1 = 9`; 3) `Ответ: 9 домиков` | 3 | — | — |
| 34 | `album` | `erasure-broken` | 1) `14 + 3 = 17`; 2) column at the side: `17 × 2 = 34`; 3) column at the side: `14 × 5 = 70`; 4) `34 + 70 = 104`; 5) `Ответ: 104 рубля` | 5 | 1: `times-as-more` | 1: correct step `14 × 3 = 42` → `14 + 3 = 17` |
| 35 | `grasshopper` | `error` | 1) `3 + 4 + 4 + 4 = 15`; 2) `15 + 5 = 20`; 3) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 → 20`; 4) `Ответ: в точке 20` | 4 | 2: `jump-direction` | — |
| 36 | `stamps` | `correct` | 1) drawing: `схема: отрезок младшего и отрезок старшего, длиннее на 6, — вместе 20`; 2) `(20 − 6) : 2 = 7`; 3) `Ответ: 7 марок` | 3 | — | — |
| 37 | `backwards` | `erasure-broken` | 1) `15 + 12 = 27`; 2) `27 + 7 = 34`; 3) drawing full width: `числовая прямая, с конца: прыжки вправо 15 → 27 → 34`; 4) `Ответ: 34` | 4 | 2: `not-inverted` | 2: correct step `27 − 7 = 20` → `27 + 7 = 34` |
| 38 | `album` | `error` | 1) column at the side: `14 + 3 = 17`; 2) column at the side: `17 × 2 = 34`; 3) column at the side: `14 × 5 = 70`; 4) `34 + 70 = 104`; 5) `Ответ: 104 рубля` | 5 | 1: `times-as-more` | — |
| 39 | `pages` | `correct` | 1) column at the side: `75 − 9 = 66`; 2) column at the side: `66 × 2 = 132`; 3) `132 + 9 = 141`; 4) `Ответ: 141 цифра` | 4 | — | — |
| 40 | `grasshopper` | `erasure-fixed` | 1) `3 + 4 + 4 + 4 = 15`; 2) `15 − 5 = 10`; 3) drawing full width: `числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10`; 4) `Ответ: в точке 10` | 4 | — | 1: mistake `3 + 4 + 4 = 11` → `3 + 4 + 4 + 4 = 15` |

## Drawings

Each drawing of the plan of code: the divisions, labels and arcs of a number line, the houses of a row, the segments of a scheme with their pieces and labels. The segments of a scheme go top to bottom, and the pieces of a segment left to right, their length in units of the scheme. For a drawing that the planned erasure touches, the table also gives the drawing itself before the erasure.

| No. | Part | Drawing | Before the erasure |
|---|---|---|---|
| 1 | 1 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 10, 11, 15; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 10 | — |
| 3 | 1 | a row of houses: 3 left of the marked one and 5 right of it, with numbers | a row of houses: 4 left of the marked one and 0 right of it, without numbers |
| 4 | 2 | line from 10 to 30 in steps of 1; labels 10, 15, 22, 27; arcs 15 → 27, 27 → 22 | line from 10 to 30 in steps of 1; labels 10, 15, 20, 27; arcs 15 → 27, 27 → 20 |
| 5 | 1 | scheme: segment □ — 7; segment △ — 7 + 6 «6»; a brace «20» | — |
| 7 | 1 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 10, 11, 15; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 10 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 13; arcs 3 → 7, 7 → 13 |
| 8 | 1 | scheme: segment □ — 7 + 6 «6»; segment △ — 7; a brace «20» | scheme: segment □ — 7; segment △ — 7 + 6 «6»; a brace «20» |
| 10 | 1 | a row of houses: 4 left of the marked one and 5 right of it, without numbers | — |
| 11 | 3 | line from 10 to 30 in steps of 1; labels 10, 15, 20, 27; arcs 15 → 27, 27 → 20 | — |
| 12 | 1 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 11, 12, 15; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 12 | — |
| 14 | 1 | scheme: segment □ — 7; segment △ — 7 + 6 «6»; a brace «20» | scheme: segment □ — 7 + 6 «6»; segment △ — 7; a brace «20» |
| 16 | 1 | line from 10 to 30 in steps of 1; labels 10, 15, 18, 25; arcs 15 → 25, 25 → 18 | — |
| 17 | 1 | a row of houses: 3 left of the marked one and 5 right of it, without numbers | — |
| 19 | 1 | a row of houses: 3 left of the marked one and 5 right of it, with numbers | — |
| 20 | 1 | scheme: segment — 7; segment — 7 + 6 «6»; a brace «20» | — |
| 22 | 1 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 10, 11, 15; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 10 | — |
| 23 | 1 | a row of houses: 3 left of the marked one and 6 right of it, with numbers | — |
| 24 | 2 | line from 10 to 30 in steps of 1; labels 10, 15, 20, 27; arcs 15 → 27, 27 → 20 | — |
| 26 | 1 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 8, 11, 13; arcs 3 → 7, 7 → 11, 11 → 13, 13 → 8 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 11, 15; arcs 3 → 7, 7 → 11, 11 → 15 |
| 27 | 1 | scheme: segment — 7; segment — 7 + 6 «6»; a brace «20» | — |
| 28 | 1 | a row of houses: 3 left of the marked one and 5 right of it, without numbers | — |
| 29 | 1 | line from 10 to 30 in steps of 1; labels 10, 15, 20, 27; arcs 15 → 27, 27 → 20 | — |
| 32 | 3 | line from 0 to 20 in steps of 1; labels 0, 3, 10, 15; arcs 15 → 3, 3 → 10 | — |
| 33 | 1 | a row of houses: 3 left of the marked one and 5 right of it, without numbers | — |
| 35 | 3 | line from 0 to 20 in steps of 1; labels 0, 3, 7, 11, 15, 20; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 20 | — |
| 36 | 1 | scheme: segment — 7; segment — 7 + 6 «6»; a brace «20» | — |
| 37 | 3 | line from 10 to 35 in steps of 1; labels 10, 15, 27, 34; arcs 15 → 27, 27 → 34 | — |
| 40 | 3 | line from 0 to 16 in steps of 1; labels 0, 3, 7, 10, 11, 15; arcs 3 → 7, 7 → 11, 11 → 15, 15 → 10 | — |
