# The sheet plan with a near key

> What code draws on the 12 sheets with a near key (T70; R36). On these sheets the result differs from the key by one similar digit: so it is seen whether the key shifts the reading (R03; Т-23). How the measurement of reading is built and how the threshold is counted — [architecture.md](architecture.md), section 3.4, "Reading into an observation".
>
> The tasks are the bank `eval/near-bank.json` in the format `tutor-bank` v0 (R15). These are copies of six tasks of the bank of E2, `eval/bank.json`, word for word, but for the false keys: each has a near-false key in place of its former false key. The bank of E2 does not change (R28). The bank and the plan are synthetic content under Apache-2.0: the executor wrote them, not learners.
>
> The plan is the file `eval/near-sheets.json` in the format `tutor-sheet-plan` v0, as the plan of E2 is ([architecture.md](architecture.md), section 3.1, "Format of the sheet plan"). The tables below are the same bank and the same plan: a test of `cmd/tutor-eval` builds them from the files and checks them against this document byte for byte, so a sheet is edited in both places.

## Tasks

The six tasks of E2. Each has a near-false key — the key with its last digit replaced by a similar one: 1 and 7, 4 and 9, 8 and 3, 5 and 6. The solution of the near-false key is the correct solution with the same digit replaced in its last line. So the key and the solution differ from the sheet by one digit, and by nothing else. That the copies equal the tasks of E2, and that the near-false keys are such, is checked by a test of `cmd/tutor-eval`, and the bank and the plan by `tutor-eval check eval/near-bank.json eval/near-sheets.json`.

| ID | Key | Near-false key | Similar digits |
|---|---|---|---|
| `chain-2` | `11` | `17` | 1 → 7 |
| `equation-3` | `x = 7` | `x = 1` | 7 → 1 |
| `equation-4` | `x = 4` | `x = 9` | 4 → 9 |
| `equation-2` | `x = 8` | `x = 3` | 8 → 3 |
| `chain-1` | `38` | `33` | 8 → 3 |
| `posts` | `5` | `6` | 5 → 6 |

0 and 6, 2 and 7 are not taken as pairs: 6 and 7 would then have two pairs each.

## How to read the plan

- **Sheet** — one task of the bank, scenario `similar-digits`. Two sheets per task:
  - **the correct one** — the lines of the correct solution. It is read also with the near-false key: so it is seen whether the model reads the key instead of what is written;
  - **the one with an error** — the same lines, but the answer is the near-false key. The first error is the answer line, there is no trap. It is read with the correct key: so it is seen whether the key gives a false "correct".
- **Lines on the sheet** — what remains on the sheet, top to bottom. The answer line is the last. For `posts` the lines are the same as on the sheets of E2 of this task, and for the others they are the lines of the bank's solution.
- **The answer's digit leans toward** — code draws the last digit of the answer line shifted toward the digit of its pair by the share `synth.SimilarShift`. Share 0 is the font's digit, 1 the pair's digit. The share is 0.1: the largest of 0.5, 0.4, 0.3, 0.2 and 0.1 at which Opus without a key read correctly no fewer than 11 sheets of the 12 of this plan, drawn with seed 2 (T73; the 2026-10-28 amendment to R36).
- **Split** — one, `tutor-eval split -seed 70 -dev 4`: 4 dev sheets and 8 test sheets. The set `synthetic/near` is drawn with seed 1 and split on 2026-10-28, when the share was frozen, — before the first edit of the prompt of reading. In the dev part are sheets 7, 9, 11 and 12: three correct ones and one with an error.

## Sheets

| No. | Task | Lines on the sheet | Outcome | The answer's digit leans toward |
|---|---|---|---|---|
| 1 | `chain-2` | `47 + 19 = 66`; `66 : 6 = 11` | `correct` | 1 → 7 |
| 2 | `chain-2` | `47 + 19 = 66`; `66 : 6 = 17` | `error` | 7 → 1 |
| 3 | `equation-3` | `9 − (x − 2) = 4`; `9 − x + 2 = 4`; `11 − x = 4`; `x = 7` | `correct` | 7 → 1 |
| 4 | `equation-3` | `9 − (x − 2) = 4`; `9 − x + 2 = 4`; `11 − x = 4`; `x = 1` | `error` | 1 → 7 |
| 5 | `equation-4` | `5x − 4 = 2x + 8`; `5x − 2x = 8 + 4`; `3x = 12`; `x = 4` | `correct` | 4 → 9 |
| 6 | `equation-4` | `5x − 4 = 2x + 8`; `5x − 2x = 8 + 4`; `3x = 12`; `x = 9` | `error` | 9 → 4 |
| 7 | `equation-2` | `2(x − 3) = 10`; `x − 3 = 5`; `x = 8` | `correct` | 8 → 3 |
| 8 | `equation-2` | `2(x − 3) = 10`; `x − 3 = 5`; `x = 3` | `error` | 3 → 8 |
| 9 | `chain-1` | `14 × 2 = 28`; `17 + 49 = 66`; `66 − 28 = 38` | `correct` | 8 → 3 |
| 10 | `chain-1` | `14 × 2 = 28`; `17 + 49 = 66`; `66 − 28 = 33` | `error` | 3 → 8 |
| 11 | `posts` | `20 : 5 = 4`; `4 + 1 = 5`; `Ответ: 5 столбов` | `correct` | 5 → 6 |
| 12 | `posts` | `20 : 5 = 4`; `4 + 1 = 5`; `Ответ: 6 столбов` | `error` | 6 → 5 |
