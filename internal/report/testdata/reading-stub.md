# The reading: the dev part

The log reading-stub.jsonl, of the prompt 94452c62dab0, version 1. Readers: opus-high (claude-opus-5-5, high, once); haiku-low (claude-haiku-5-5, low); haiku-low-no-thinking (claude-haiku-5-5, low, without thinking). Repeats: 2. Reads: keys, heights, states. Sheets: 7 (lines 4, free 3). Calls: 198 made, 0 of them failed; $0.24 spent of the $10.00 cap.

## Does the key make the reading worse? The threshold of R03 (Т-23)

| Reader | Sheets | Σd | Sheets with d ≠ 0 | p, worse | Under the right key | Under a false key, on correct sheets | Sheets with a new false "correct" | Under no key, against the right key | One call admissible |
|---|---|---|---|---|---|---|---|---|---|
| opus-high | 7 | +0.00 | 0 | 1 | 0 of 7 | 0 of 4 | 0 | 0 of 7 | the threshold holds here; only the test part decides |
| haiku-low | 7 | +0.00 | 0 | 1 | 0 of 14 | 0 of 8 | 0 | 0 of 14 | the threshold holds here; only the test part decides |
| haiku-low-no-thinking | 7 | +0.00 | 0 | 1 | 0 of 14 | 0 of 8 | 0 | 0 of 14 | the threshold holds here; only the test part decides |

d of a sheet is the share of its false "correct" and false alarms with the right key less that without a key, over the repeats whose calls did not fail; a false "correct" is "correct" on a sheet whose outcome is not, and a false alarm "not yet" on a correct sheet. The test is the exact one-sided sign-permutation test over the sheets, and the key makes the reading worse at p < 0.05. One call is admissible for a reader, with its admission by eval, only if the key makes nothing worse, no call with the right key and none with a false key on a correct sheet reads anything under the key, and no sheet has a false "correct" with the right key where it had none without a key (R03). What is read under the right key without a key is the baseline, outside the threshold. Only the test part decides (R03).

With fewer than 5 sheets with d ≠ 0 the test cannot find the reading worse: opus-high has 0; haiku-low has 0; haiku-low-no-thinking has 0.

## Readings of the finished sheets at 32 px

| Reader | Condition | Calls | Right | False "correct" | False alarms | Extra "unreadable" | Failed | Under the key | Parts of writing read |
|---|---|---|---|---|---|---|---|---|---|
| opus-high | none | 7 | 7 | 0 of 3, ≤ 0.632 | 0 of 4, ≤ 0.527 | 0 of 7, ≤ 0.348 | 0 | 0 | 21 of 21 |
| opus-high | right | 7 | 7 | 0 of 3, ≤ 0.632 | 0 of 4, ≤ 0.527 | 0 of 7, ≤ 0.348 | 0 | 0 | 21 of 21 |
| opus-high | false | 4 | 4 | 0 of 0, ≤ 1.000 | 0 of 4, ≤ 0.527 | 0 of 4, ≤ 0.527 | 0 | 0 | 10 of 10 |
| haiku-low | none | 14 | 14 | 0 of 6, ≤ 0.393 | 0 of 8, ≤ 0.312 | 0 of 14, ≤ 0.193 | 0 | 0 | 42 of 42 |
| haiku-low | right | 14 | 14 | 0 of 6, ≤ 0.393 | 0 of 8, ≤ 0.312 | 0 of 14, ≤ 0.193 | 0 | 0 | 42 of 42 |
| haiku-low | false | 8 | 8 | 0 of 0, ≤ 1.000 | 0 of 8, ≤ 0.312 | 0 of 8, ≤ 0.312 | 0 | 0 | 20 of 20 |
| haiku-low-no-thinking | none | 14 | 14 | 0 of 6, ≤ 0.393 | 0 of 8, ≤ 0.312 | 0 of 14, ≤ 0.193 | 0 | 0 | 42 of 42 |
| haiku-low-no-thinking | right | 14 | 14 | 0 of 6, ≤ 0.393 | 0 of 8, ≤ 0.312 | 0 of 14, ≤ 0.193 | 0 | 0 | 42 of 42 |
| haiku-low-no-thinking | false | 8 | 8 | 0 of 0, ≤ 1.000 | 0 of 8, ≤ 0.312 | 0 of 8, ≤ 0.312 | 0 | 0 | 20 of 20 |

A share's bound is the one-sided upper bound of 95 % over the sheets, Clopper–Pearson on n / Deff. False "correct" are counted among the readings that did not fail of sheets whose outcome is not correct, and false alarms among those of correct sheets; extra "unreadable" among all the readings of sheets whose outcome is correct or an error, a failed one counted as one (R08). A part of writing is read when the observation has it as written, compared without spaces and after normalization.

## Two readers (R08; Т-33)

Without a key, the finished sheets at 32 px, the first repeat.

| Readers | Sheets | Released | Released, false "correct" | Released, false alarms | Released, extra "unreadable" | The strong alone | The fast alone | The fast says "correct" | The strong too | p of the hybrid |
|---|---|---|---|---|---|---|---|---|---|---|
| opus-high and haiku-low | 7 | 7 | 0 | 0 | 0 | 0 / 0 / 0 | 0 / 0 / 0 | 4 | 4 | 0.429 |
| opus-high and haiku-low-no-thinking | 7 | 7 | 0 | 0 | 0 | 0 / 0 / 0 | 0 / 0 / 0 | 4 | 4 | 0.429 |

A sheet is released when both readers give one result and its values, read as the check of the kind number reads them, and their verdicts are one; otherwise it is "unreadable". A reader alone is counted as false "correct" / false alarms / extra "unreadable" on the same sheets. p of the hybrid is the share of sheets the fast reader does not confirm "correct" on, which go on to the strong one (architecture 2.6).

## Heights of digits (R08; R51 spec)

Without a key, the finished sheets in the first repeat, each height against 32 px on the same sheets.

| Reader | Height, px | Sheets | Errors at the height | Errors at 32 px | Σd | Sheets with d ≠ 0 | p, worse | New false "correct" | Drawn smaller, to fit | Not worse than 32 px |
|---|---|---|---|---|---|---|---|---|---|---|
| haiku-low | 16, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 16, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 16, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 24, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 24, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 24, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 48, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 48, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low | 48, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 16, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 16, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 16, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 24, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 24, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 24, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 48, all the sheets | 7 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 48, without small letters | 1 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |
| haiku-low-no-thinking | 48, with small letters | 6 | 0 | 0 | +0 | 0 | 1 | 0 | 0 | yes |

An error is a false "correct", a false alarm or an extra "unreadable", a failed call counted as one. A height is not worse than 32 px if the exact one-sided sign-permutation test over the sheets does not find it worse at p < 0.05 and no sheet has a false "correct" at it where it had none at 32 px. The advice is the smallest of 16 and 24 px not worse on all the sheets for every reader that read the heights, and otherwise 32 px. On sheets with small letters the crop's estimate may fall to the height of a small letter, and the digits come out larger than asked (В36).

Here the advice would be 16 px; only the test part decides.

## States: the reading of unfinished work (О-8, О-21)

Without a key, at 32 px. A state of two kinds counts in both.

| Reader | Kind | Calls | Failed | "Correct" or "not yet" before the answer is written | The answer written: right | The answer written, with what is erased later | Parts written whole, read as written | Results before the answer is written | Begun parts read as if written whole |
|---|---|---|---|---|---|---|---|---|---|
| haiku-low | after a line | 19 | 0 | 0 of 19, ≤ 0.146 | 0 of 0 | 0 | 25 of 25 | 0 of 19 | 0 of 0 |
| haiku-low | at a pause | 14 | 0 | 0 of 14, ≤ 0.193 | 0 of 0 | 0 | 16 of 16 | 0 of 14 | 0 of 12 |
| haiku-low | before an erasure | 1 | 0 | 0 of 1, ≤ 0.950 | 0 of 0 | 0 | 0 of 0 | 0 of 1 | 0 of 0 |
| haiku-low-no-thinking | after a line | 19 | 0 | 0 of 19, ≤ 0.146 | 0 of 0 | 0 | 25 of 25 | 0 of 19 | 0 of 0 |
| haiku-low-no-thinking | at a pause | 14 | 0 | 0 of 14, ≤ 0.193 | 0 of 0 | 0 | 16 of 16 | 0 of 14 | 0 of 12 |
| haiku-low-no-thinking | before an erasure | 1 | 0 | 0 of 1, ≤ 0.950 | 0 of 0 | 0 | 0 of 0 | 0 of 1 | 0 of 0 |

The answer is written when every stroke of its part is. A verdict before that is out of place, its share bounded over the sheets; so is a result given then, which the work does not yet give. A state that still shows what is erased later is not scored for its verdict: the markup does not tell what it shows. All but the calls are counted over the calls that did not fail.

## Readings of a sheet by each policy of reading on its own (О-8)

Beyond the sending, from the strokes and erasures alone (R10).

| Policy | Mean a sheet | Median | Most | opus-high, $ a sheet | haiku-low, $ a sheet | haiku-low-no-thinking, $ a sheet |
|---|---|---|---|---|---|---|
| after each line | 2.71 | 3 | 5 | 0.02655 | 0.00105 | 0.00064 |
| at each pause of 0.4 s | 14.43 | 12 | 27 | 0.14112 | 0.00557 | 0.00340 |
| at each pause of 2 s | 2.29 | 3 | 4 | 0.02236 | 0.00088 | 0.00054 |
| at each pause of 6 s | 0.29 | 0 | 2 | 0.00279 | 0.00011 | 0.00007 |

A sheet's price by a policy is its mean readings times the mean cost of a call: opus-high by its readings without a key, $0.00978; haiku-low by its states, $0.00039; haiku-low-no-thinking by its states, $0.00024.

## Time and cost of a call

| Reader | Condition | Time p50, ms | Time p95, ms | Cost, $ |
|---|---|---|---|---|
| opus-high | none | 0 | 0 | 0.00978 |
| opus-high | right | 0 | 0 | 0.01038 |
| opus-high | false | 0 | 0 | 0.01066 |
| haiku-low | none | 0 | 0 | 0.00039 |
| haiku-low | right | 0 | 0 | 0.00041 |
| haiku-low | false | 0 | 0 | 0.00042 |
| haiku-low-no-thinking | none | 0 | 0 | 0.00024 |
| haiku-low-no-thinking | right | 0 | 0 | 0.00026 |
| haiku-low-no-thinking | false | 0 | 0 | 0.00027 |
