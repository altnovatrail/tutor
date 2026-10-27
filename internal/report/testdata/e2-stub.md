# The measurement of the writing process: the test part

The log e2-stub.jsonl, of the prompt 6a98139b3dbd. Judges: opus-high (claude-opus-5-5, high); haiku-low (claude-haiku-5-5, low); haiku-low-no-thinking (claude-haiku-5-5, low, without thinking). Repeats: 2. Sheets: 7, 2 of them with frames. Calls: 108 made, 0 of them failed; $0.39 spent of the $10.00 cap.

## Does the writing process help? Form D against form A

| Judge | Σd | Sheets with d ≠ 0 | p, helps | p, harms | False "correct", A → D | False alarms, A → D | Outcome | Passes Holm |
|---|---|---|---|---|---|---|---|---|
| opus-high | +0.00 | 0 | 1 | 1 | 0 → 0 | 0 → 0 | not shown | no |
| haiku-low | +0.00 | 0 | 1 | 1 | 0 → 0 | 0 → 0 | not shown | no |
| haiku-low-no-thinking | +0.00 | 0 | 1 | 1 | 0 → 0 | 0 → 0 | not shown | no |

d of a sheet is the share of right answers in form D less that in form A; on a sheet without frames, form D is form B. A right answer is "correct" on a correct sheet, and on a sheet with a mistake "error" with the box on the line of the first error; a failed call is a wrong answer. The test is the exact one-sided sign-permutation test over the sheets, at p < 0.05; a judge helps if it passes, with no more false "correct" and no more false alarms in D than in A, and harms the other way round.

With fewer than 5 sheets with d ≠ 0 a judge cannot pass the test: opus-high has 0, and its smallest p is 1/1; haiku-low has 0, and its smallest p is 1/1; haiku-low-no-thinking has 0, and its smallest p is 1/1.

**That the writing process helps Claude is not shown:** no judge that helps passes Holm's correction for 3 judges, p < 0.0167.

## Without a threshold

### On the sheets with frames

| Judge | Forms | Σd | Sheets with d ≠ 0 | p, helps | p, harms |
|---|---|---|---|---|---|
| opus-high | B against A | +0.00 | 0 | 1 | 1 |
| opus-high | C against A | +0.00 | 0 | 1 | 1 |
| opus-high | D against B | +0.00 | 0 | 1 | 1 |
| haiku-low | B against A | +0.00 | 0 | 1 | 1 |
| haiku-low | C against A | +0.00 | 0 | 1 | 1 |
| haiku-low | D against B | +0.00 | 0 | 1 | 1 |
| haiku-low-no-thinking | B against A | +0.00 | 0 | 1 | 1 |
| haiku-low-no-thinking | C against A | +0.00 | 0 | 1 | 1 |
| haiku-low-no-thinking | D against B | +0.00 | 0 | 1 | 1 |

### Answers by judge and form

| Judge | Form | Answers | Right | False "correct" | False alarms | "Unreadable" | Failed | Repeats that disagree |
|---|---|---|---|---|---|---|---|---|
| opus-high | A | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| opus-high | B | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| opus-high | C | 4 | 4 | 0 | 0 | 0 | 0 | 0 of 2 sheets |
| opus-high | D | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low | A | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low | B | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low | C | 4 | 4 | 0 | 0 | 0 | 0 | 0 of 2 sheets |
| haiku-low | D | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low-no-thinking | A | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low-no-thinking | B | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |
| haiku-low-no-thinking | C | 4 | 4 | 0 | 0 | 0 | 0 | 0 of 2 sheets |
| haiku-low-no-thinking | D | 14 | 14 | 0 | 0 | 0 | 0 | 0 of 7 sheets |

### Erasures

| Judge | Form | False alarms on erasure-fixed | Right on erasure-broken |
|---|---|---|---|
| opus-high | A | 0 of 2 | 2 of 2 |
| opus-high | B | 0 of 2 | 2 of 2 |
| opus-high | C | 0 of 2 | 2 of 2 |
| opus-high | D | 0 of 2 | 2 of 2 |
| haiku-low | A | 0 of 2 | 2 of 2 |
| haiku-low | B | 0 of 2 | 2 of 2 |
| haiku-low | C | 0 of 2 | 2 of 2 |
| haiku-low | D | 0 of 2 | 2 of 2 |
| haiku-low-no-thinking | A | 0 of 2 | 2 of 2 |
| haiku-low-no-thinking | B | 0 of 2 | 2 of 2 |
| haiku-low-no-thinking | C | 0 of 2 | 2 of 2 |
| haiku-low-no-thinking | D | 0 of 2 | 2 of 2 |

### Time and cost of a call

| Judge | Form | Time p50, ms | Time p95, ms | First token p50, ms | First token p95, ms | Cost, $ |
|---|---|---|---|---|---|---|
| opus-high | A | 0 | 0 | 0 | 0 | 0.00840 |
| opus-high | B | 0 | 0 | 0 | 0 | 0.01000 |
| opus-high | C | 0 | 0 | 0 | 0 | 0.01332 |
| opus-high | D | 0 | 0 | 0 | 0 | 0.01492 |
| haiku-low | A | 0 | 0 | 0 | 0 | 0.00034 |
| haiku-low | B | 0 | 0 | 0 | 0 | 0.00038 |
| haiku-low | C | 0 | 0 | 0 | 0 | 0.00046 |
| haiku-low | D | 0 | 0 | 0 | 0 | 0.00050 |
| haiku-low-no-thinking | A | 0 | 0 | 0 | 0 | 0.00021 |
| haiku-low-no-thinking | B | 0 | 0 | 0 | 0 | 0.00025 |
| haiku-low-no-thinking | C | 0 | 0 | 0 | 0 | 0.00033 |
| haiku-low-no-thinking | D | 0 | 0 | 0 | 0 | 0.00037 |

### Agreement with opus-high in "correct"

| Judge | Says "correct" | opus-high says "correct" too | Says "unreadable" |
|---|---|---|---|
| haiku-low | 24 | 24 | 0 of 36 |
| haiku-low-no-thinking | 24 | 24 | 0 of 36 |
