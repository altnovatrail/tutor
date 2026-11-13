# Pages of the workbook

> What the code writes on 20 synthetic pages of the workbook (T78; R40). On them the measurement E4 measures how Claude reads handwritten answers in the boxes and how the code gives each item a verdict by what was read (R41 spec; the 2026-10-27 amendment to R53 spec). How the reading call and the report are built — [architecture.md](architecture.md), section 3.4, "Workbook".
>
> The pages are the file `eval/workbook.json` in the format `tutor-workbook` v0 (R40). This is synthetic content under Apache-2.0: the items and the answers in the boxes were written by the executor's code, not by learners. There are no files of pages: the code draws the strokes in memory, from the file and the seed. The tables below are the same file: a test of `cmd/tutor-eval` builds them from the file and compares them with this document byte for byte, so a box is edited in both places.

## How to read the tables

- **Page** — 10 items and 11 boxes. The items are always of the same kinds, and each page has its own numbers:
  1. multiplication with the box in place of a factor, "6 × [ ] = 30";
  2. division, "30 : 5 = [ ]";
  3. an unknown addend, "[ ] + 27 = 64";
  4. subtraction of three-digit numbers with borrowing, "639 − 472 = [ ]";
  5. division with a remainder — two boxes, the quotient and the remainder, «17 : 5 = [ ] (ост. [ ])»;
  6. addition of fractions with the same denominator;
  7. subtraction of fractions with the same denominator;
  8. a fraction of an hour, a kilogram or a metre, «25 см — это [ ] метра»;
  9. reducing a fraction, «Сократи дробь: 8/14 = [ ]»;
  10. addition of fractions, the answer as a fraction in lowest terms.
- **Item line** — as the learner sees it: `[ ]` is a box. A fraction in the line is written inline, as in the format of `spec` v0: an item line has no stacked fractions yet (R51 spec).
- **Box** — the place of the box among the item's boxes, from 1. Together with the item's number, it is the place of the answer slot in the reading's response, `{ item, slot }`.
- **Check** — the check kind of the box by the format of `spec` (R56 spec):
  - **number** — the value is compared with the key as a rational number, exactly;
  - **fraction** — any simple fraction of the same value is correct: `2/10` and `1/5` are the same;
  - **fraction in lowest terms** — only a fraction in lowest terms is correct. A reducible fraction of the same value is "not yet".
- **Key** and **traps** — as in a module: a trap is a wrong value that the author knows, and the name of the error it shows. The values of traps are "not yet".
- **Written** — what the code writes in the box, as a person would read it. A fraction the code writes **stacked** — the numerator, a bar, the denominator — or **inline**, with a slash. "Crossed out" is a wrong value struck through with a stroke, with the correct one written beside it in the same box. "—" means the box is blank.
- **Scenario** — why the box is written so:
  - **correct** — the key, with nothing special;
  - **error** — a typical error, the value of the first trap;
  - **near key** — a wrong value that differs from the key by one digit of a pair of similar ones: 1 and 7, 4 and 9, 8 and 3, 5 and 6. "8 → 3" means: 8 is written, and this digit leans slightly toward the 3 of the key. This shows whether the model reads the key that the item line suggests instead of what is written;
  - **similar digits** — the key, but its last digit, one of such a pair, leans slightly toward the other digit of its pair, "1 → 7". This shows whether there is a false alarm;
  - **mirrored digit** — the key, but the first of its digits 2, 5, 7 and 9 is written mirrored, as children write it. The truth is the same digit, and a reading by the rule of `spec` `v0.3.3` does not return it as a digit: the box is "unreadable", and the learner rewrites it (the 2026-10-29 amendment to R56 spec; T99);
  - **crossed out** — an error is crossed out and corrected; what must be read is what is not crossed out;
  - **blank** — nothing is written.

  The lean is the same share as on the sheets with a near key, `synth.SimilarShift`, 0.1 (R36, the 2026-10-28 amendment).
- **Verdict** — what a correct reading of the box comes to: the check kind applied to what is written. "Not yet" names the trap if the value is that trap's. A blank box is "unreadable" with the cause `blank`: it has no crop, and it does not go to the model (the 2026-10-27 amendment to R53 spec). An item's verdict is made up of the verdicts of its boxes.

## How a page is built

- **Page** — 1000 × 1400 units, a box 100 × 100: these are the layout numbers of `canvas`, R35 canvas. The layout here is simpler than in `canvas`: each item is a row of its own, 100 high, the rows 32 apart, the margins 40, and the box stands after the text before it, by the widths of the printed characters at size 30.
- **Handwriting** — each page has its own, from the seed and the page number: the size, the slant and the speed of the pen. In a box the writing is fitted to the box and does not go out of it. So by the box rule of `canvas`, R37 canvas, each stroke belongs to a box of its own, and the page has no draft. A test checks this.
- **Crop** — for each filled box, by its strokes, by the crop rule of the eval protocol, R25 canvas: only the ink, without the outline of the box and the text of the line. At 16, 24 and 32 px a crop is 200 × 200, 64 tokens, and a page 640 tokens. At 48 px ten crops come out wider, up to 265 px: these are inline fractions with two-digit parts and crossed-out ones.
- **To view** the crops at 32 px — what the model will see — run `go run ./cmd/tutor-eval workbook -png synthetic/png/workbook`. The directory is in `synthetic/`, outside git. *2026-11-07 (T107; R71): the pages are samples of the eval protocol, and `draw` saves the crops as it draws them: `go run ./cmd/tutor-eval draw -workbook eval/workbook.json -dir synthetic/png/pages -png synthetic/png/workbook`, into a directory with no pages in it yet.*
- **Split** — pages 1–5 are the dev part, 6–20 the test part (T80).

## How many boxes of each kind

| Scenario | Boxes | Number | Fraction | Of them in lowest terms | Stacked | Inline | Dev | Test |
|---|---|---|---|---|---|---|---|---|
| correct | 100 | 53 | 47 | 20 | 23 | 24 | 25 | 75 |
| error | 20 | 12 | 8 | 4 | 3 | 5 | 5 | 15 |
| near key | 20 | 13 | 7 | 2 | 4 | 3 | 5 | 15 |
| similar digits | 20 | 9 | 11 | 3 | 4 | 7 | 5 | 15 |
| mirrored digit | 20 | 9 | 11 | 4 | 8 | 3 | 5 | 15 |
| crossed out | 20 | 13 | 7 | 2 | 4 | 3 | 5 | 15 |
| blank | 20 | 11 | 9 | 5 | 0 | 0 | 5 | 15 |
| **total** | 220 | 120 | 100 | 40 | 46 | 45 | 55 | 165 |

Dev and Test are the numbers of boxes on pages 1–5 and on pages 6–20.

## Boxes

| Page | No. | Item line | Box | Check | Key | Traps | Written | Scenario | Verdict |
|---|---|---|---|---|---|---|---|---|---|
| 1 | 1 | `6 × [ ] = 30` | 1 | number | `5` | `24` wrong-operation | `5` | correct | correct |
| 1 | 2 | `30 : 5 = [ ]` | 1 | number | `6` | `7` table-neighbour | — | blank | unreadable, blank |
| 1 | 3 | `[ ] + 22 = 52` | 1 | number | `30` | `74` adds-instead | `30` | correct | correct |
| 1 | 4 | `639 − 472 = [ ]` | 1 | number | `167` | `247` no-borrow | `167` | correct | correct |
| 1 | 5 | `17 : 5 = [ ] (ост. [ ])` | 1 | number | `3` | `2` quotient-too-small | `8` | near key: 8 → 3 | not yet |
| 1 | 5 | `17 : 5 = [ ] (ост. [ ])` | 2 | number | `2` | `7` remainder-too-big | `2` | correct | correct |
| 1 | 6 | `7/11 + 1/11 = [ ]` | 1 | fraction | `8/11` | `8/22` adds-denominators | `8/11` stacked | similar digits: 1 → 7 | correct |
| 1 | 7 | `4/10 − 2/10 = [ ]` | 1 | fraction | `2/10` | `6/10` adds-instead | `2/10` inline | correct | correct |
| 1 | 8 | `25 см — это [ ] метра` | 1 | fraction | `1/4` | `25/10` ten-centimetres | `25/10` stacked | error | not yet, ten-centimetres |
| 1 | 9 | `Сократи дробь: 8/14 = [ ]` | 1 | fraction in lowest terms | `4/7` | `8/14` not-in-lowest-terms, `4/14` reduces-numerator-only | `8/14` inline crossed out, `4/7` inline | crossed out | correct |
| 1 | 10 | `3/10 + 3/10 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `3/5` | `6/10` not-in-lowest-terms, `6/20` adds-denominators | `3/5` stacked | mirrored digit: 5 | correct |
| 2 | 1 | `7 × [ ] = 56` | 1 | number | `8` | `49` wrong-operation | `8` | correct | correct |
| 2 | 2 | `18 : 6 = [ ]` | 1 | number | `3` | `4` table-neighbour | `3` | correct | correct |
| 2 | 3 | `[ ] + 29 = 61` | 1 | number | `32` | `90` adds-instead | — | blank | unreadable, blank |
| 2 | 4 | `527 − 232 = [ ]` | 1 | number | `295` | `315` no-borrow | `296` | near key: 6 → 5 | not yet |
| 2 | 5 | `29 : 4 = [ ] (ост. [ ])` | 1 | number | `7` | `6` quotient-too-small | `7` | correct | correct |
| 2 | 5 | `29 : 4 = [ ] (ост. [ ])` | 2 | number | `1` | `5` remainder-too-big | `1` | correct | correct |
| 2 | 6 | `4/8 + 1/8 = [ ]` | 1 | fraction | `5/8` | `5/16` adds-denominators | `5/16` inline | error | not yet, adds-denominators |
| 2 | 7 | `6/7 − 3/7 = [ ]` | 1 | fraction | `3/7` | `9/7` adds-instead | `3/7` stacked | correct | correct |
| 2 | 8 | `200 г — это [ ] кг` | 1 | fraction | `1/5` | `200/100` hundred-grams | `200/100` inline crossed out, `1/5` inline | crossed out | correct |
| 2 | 9 | `Сократи дробь: 14/16 = [ ]` | 1 | fraction in lowest terms | `7/8` | `14/16` not-in-lowest-terms, `7/16` reduces-numerator-only | `7/8` stacked | mirrored digit: 7 | correct |
| 2 | 10 | `1/6 + 1/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/3` | `2/6` not-in-lowest-terms, `2/12` adds-denominators | `1/3` inline | similar digits: 3 → 8 | correct |
| 3 | 1 | `6 × [ ] = 54` | 1 | number | `9` | `48` wrong-operation | `9` | correct | correct |
| 3 | 2 | `6 : 3 = [ ]` | 1 | number | `2` | `3` table-neighbour | `2` | correct | correct |
| 3 | 3 | `[ ] + 38 = 62` | 1 | number | `24` | `100` adds-instead | `29` | near key: 9 → 4 | not yet |
| 3 | 4 | `320 − 206 = [ ]` | 1 | number | `114` | `126` no-borrow | — | blank | unreadable, blank |
| 3 | 5 | `28 : 9 = [ ] (ост. [ ])` | 1 | number | `3` | `2` quotient-too-small | `2` crossed out, `3` | crossed out | correct |
| 3 | 5 | `28 : 9 = [ ] (ост. [ ])` | 2 | number | `1` | `10` remainder-too-big | `1` | correct | correct |
| 3 | 6 | `3/10 + 2/10 = [ ]` | 1 | fraction | `5/10` | `5/20` adds-denominators | `5/10` stacked | correct | correct |
| 3 | 7 | `5/11 − 3/11 = [ ]` | 1 | fraction | `2/11` | `8/11` adds-instead | `2/11` inline | correct | correct |
| 3 | 8 | `4 см — это [ ] метра` | 1 | fraction | `1/25` | `4/10` ten-centimetres | `1/25` stacked | similar digits: 5 → 6 | correct |
| 3 | 9 | `Сократи дробь: 4/16 = [ ]` | 1 | fraction in lowest terms | `1/4` | `4/16` not-in-lowest-terms, `1/16` reduces-numerator-only | `4/16` inline | error | not yet, not-in-lowest-terms |
| 3 | 10 | `1/6 + 3/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/3` | `4/6` not-in-lowest-terms, `4/12` adds-denominators | `2/3` stacked | mirrored digit: 2 | correct |
| 4 | 1 | `3 × [ ] = 12` | 1 | number | `4` | `9` wrong-operation | `4` | correct | correct |
| 4 | 2 | `14 : 7 = [ ]` | 1 | number | `2` | `3` table-neighbour | `3` | error | not yet, table-neighbour |
| 4 | 3 | `[ ] + 25 = 53` | 1 | number | `28` | `78` adds-instead | `28` | similar digits: 8 → 3 | correct |
| 4 | 4 | `377 − 186 = [ ]` | 1 | number | `191` | `211` no-borrow | `191` | mirrored digit: 9 | correct |
| 4 | 5 | `54 : 8 = [ ] (ост. [ ])` | 1 | number | `6` | `5` quotient-too-small | `6` | correct | correct |
| 4 | 5 | `54 : 8 = [ ] (ост. [ ])` | 2 | number | `6` | `14` remainder-too-big | — | blank | unreadable, blank |
| 4 | 6 | `7/9 + 1/9 = [ ]` | 1 | fraction | `8/9` | `8/18` adds-denominators | `8/4` inline | near key: 4 → 9 | not yet |
| 4 | 7 | `4/5 − 2/5 = [ ]` | 1 | fraction | `2/5` | `6/5` adds-instead | `6/5` stacked crossed out, `2/5` stacked | crossed out | correct |
| 4 | 8 | `12 минут — это [ ] часа` | 1 | fraction | `1/5` | `12/100` hundred-minutes | `1/5` inline | correct | correct |
| 4 | 9 | `Сократи дробь: 6/12 = [ ]` | 1 | fraction in lowest terms | `1/2` | `6/12` not-in-lowest-terms, `1/12` reduces-numerator-only | `1/2` stacked | correct | correct |
| 4 | 10 | `1/4 + 1/4 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/2` | `2/4` not-in-lowest-terms, `2/8` adds-denominators | `1/2` inline | correct | correct |
| 5 | 1 | `4 × [ ] = 36` | 1 | number | `9` | `32` wrong-operation | `9` | correct | correct |
| 5 | 2 | `28 : 4 = [ ]` | 1 | number | `7` | `8` table-neighbour | `7` | correct | correct |
| 5 | 3 | `[ ] + 30 = 71` | 1 | number | `41` | `101` adds-instead | — | blank | unreadable, blank |
| 5 | 4 | `707 − 192 = [ ]` | 1 | number | `515` | `695` no-borrow | `695` crossed out, `515` | crossed out | correct |
| 5 | 5 | `52 : 6 = [ ] (ост. [ ])` | 1 | number | `8` | `7` quotient-too-small | `8` | correct | correct |
| 5 | 5 | `52 : 6 = [ ] (ост. [ ])` | 2 | number | `4` | `10` remainder-too-big | `10` | error | not yet, remainder-too-big |
| 5 | 6 | `2/8 + 5/8 = [ ]` | 1 | fraction | `7/8` | `7/16` adds-denominators | `7/3` stacked | near key: 3 → 8 | not yet |
| 5 | 7 | `3/5 − 1/5 = [ ]` | 1 | fraction | `2/5` | `4/5` adds-instead | `2/5` inline | similar digits: 5 → 6 | correct |
| 5 | 8 | `30 минут — это [ ] часа` | 1 | fraction | `1/2` | `30/100` hundred-minutes | `1/2` stacked | mirrored digit: 2 | correct |
| 5 | 9 | `Сократи дробь: 6/10 = [ ]` | 1 | fraction in lowest terms | `3/5` | `6/10` not-in-lowest-terms, `3/10` reduces-numerator-only | `3/5` inline | correct | correct |
| 5 | 10 | `3/12 + 7/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `5/6` | `10/12` not-in-lowest-terms, `10/24` adds-denominators | `5/6` stacked | correct | correct |
| 6 | 1 | `3 × [ ] = 15` | 1 | number | `5` | `12` wrong-operation | `5` | mirrored digit: 5 | correct |
| 6 | 2 | `24 : 4 = [ ]` | 1 | number | `6` | `7` table-neighbour | `6` | correct | correct |
| 6 | 3 | `[ ] + 22 = 55` | 1 | number | `33` | `77` adds-instead | `33` | correct | correct |
| 6 | 4 | `306 − 193 = [ ]` | 1 | number | `113` | `293` no-borrow | `113` | correct | correct |
| 6 | 5 | `88 : 9 = [ ] (ост. [ ])` | 1 | number | `9` | `8` quotient-too-small | `9` | correct | correct |
| 6 | 5 | `88 : 9 = [ ] (ост. [ ])` | 2 | number | `7` | `16` remainder-too-big | `16` crossed out, `7` | crossed out | correct |
| 6 | 6 | `4/8 + 2/8 = [ ]` | 1 | fraction | `6/8` | `6/16` adds-denominators | `6/3` inline | near key: 3 → 8 | not yet |
| 6 | 7 | `8/10 − 7/10 = [ ]` | 1 | fraction | `1/10` | `15/10` adds-instead | — | blank | unreadable, blank |
| 6 | 8 | `50 см — это [ ] метра` | 1 | fraction | `1/2` | `50/10` ten-centimetres | `50/10` stacked | error | not yet, ten-centimetres |
| 6 | 9 | `Сократи дробь: 6/24 = [ ]` | 1 | fraction in lowest terms | `1/4` | `6/24` not-in-lowest-terms, `1/24` reduces-numerator-only | `1/4` inline | similar digits: 4 → 9 | correct |
| 6 | 10 | `3/12 + 1/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/3` | `4/12` not-in-lowest-terms, `4/24` adds-denominators | `1/3` stacked | correct | correct |
| 7 | 1 | `7 × [ ] = 28` | 1 | number | `4` | `21` wrong-operation | `9` | near key: 9 → 4 | not yet |
| 7 | 2 | `9 : 3 = [ ]` | 1 | number | `3` | `4` table-neighbour | `3` | similar digits: 3 → 8 | correct |
| 7 | 3 | `[ ] + 14 = 49` | 1 | number | `35` | `63` adds-instead | `35` | mirrored digit: 5 | correct |
| 7 | 4 | `312 − 115 = [ ]` | 1 | number | `197` | `203` no-borrow | `197` | correct | correct |
| 7 | 5 | `57 : 9 = [ ] (ост. [ ])` | 1 | number | `6` | `5` quotient-too-small | `5` | error | not yet, quotient-too-small |
| 7 | 5 | `57 : 9 = [ ] (ост. [ ])` | 2 | number | `3` | `12` remainder-too-big | — | blank | unreadable, blank |
| 7 | 6 | `1/8 + 4/8 = [ ]` | 1 | fraction | `5/8` | `5/16` adds-denominators | `5/8` inline | correct | correct |
| 7 | 7 | `8/10 − 5/10 = [ ]` | 1 | fraction | `3/10` | `13/10` adds-instead | `13/10` stacked crossed out, `3/10` stacked | crossed out | correct |
| 7 | 8 | `100 г — это [ ] кг` | 1 | fraction | `1/10` | `100/100` hundred-grams | `1/10` inline | correct | correct |
| 7 | 9 | `Сократи дробь: 4/18 = [ ]` | 1 | fraction in lowest terms | `2/9` | `4/18` not-in-lowest-terms, `2/18` reduces-numerator-only | `2/9` stacked | correct | correct |
| 7 | 10 | `4/10 + 4/10 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `4/5` | `8/10` not-in-lowest-terms, `8/20` adds-denominators | `4/5` inline | correct | correct |
| 8 | 1 | `4 × [ ] = 20` | 1 | number | `5` | `16` wrong-operation | `5` | mirrored digit: 5 | correct |
| 8 | 2 | `12 : 4 = [ ]` | 1 | number | `3` | `4` table-neighbour | `4` | error | not yet, table-neighbour |
| 8 | 3 | `[ ] + 20 = 57` | 1 | number | `37` | `77` adds-instead | — | blank | unreadable, blank |
| 8 | 4 | `642 − 515 = [ ]` | 1 | number | `127` | `133` no-borrow | `127` | correct | correct |
| 8 | 5 | `36 : 8 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | `9` | near key: 9 → 4 | not yet |
| 8 | 5 | `36 : 8 = [ ] (ост. [ ])` | 2 | number | `4` | `12` remainder-too-big | `12` crossed out, `4` | crossed out | correct |
| 8 | 6 | `5/12 + 1/12 = [ ]` | 1 | fraction | `6/12` | `6/24` adds-denominators | `6/12` stacked | correct | correct |
| 8 | 7 | `5/9 − 3/9 = [ ]` | 1 | fraction | `2/9` | `8/9` adds-instead | `2/9` inline | similar digits: 9 → 4 | correct |
| 8 | 8 | `36 минут — это [ ] часа` | 1 | fraction | `3/5` | `36/100` hundred-minutes | `3/5` stacked | correct | correct |
| 8 | 9 | `Сократи дробь: 5/10 = [ ]` | 1 | fraction in lowest terms | `1/2` | `5/10` not-in-lowest-terms, `1/10` reduces-numerator-only | `1/2` inline | correct | correct |
| 8 | 10 | `3/10 + 1/10 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/5` | `4/10` not-in-lowest-terms, `4/20` adds-denominators | `2/5` stacked | correct | correct |
| 9 | 1 | `5 × [ ] = 30` | 1 | number | `6` | `25` wrong-operation | `25` | error | not yet, wrong-operation |
| 9 | 2 | `36 : 6 = [ ]` | 1 | number | `6` | `7` table-neighbour | `6` | correct | correct |
| 9 | 3 | `[ ] + 24 = 38` | 1 | number | `14` | `62` adds-instead | `14` | correct | correct |
| 9 | 4 | `381 − 253 = [ ]` | 1 | number | `128` | `132` no-borrow | `128` | mirrored digit: 2 | correct |
| 9 | 5 | `48 : 5 = [ ] (ост. [ ])` | 1 | number | `9` | `8` quotient-too-small | `9` | correct | correct |
| 9 | 5 | `48 : 5 = [ ] (ост. [ ])` | 2 | number | `3` | `8` remainder-too-big | `8` crossed out, `3` | crossed out | correct |
| 9 | 6 | `5/11 + 4/11 = [ ]` | 1 | fraction | `9/11` | `9/22` adds-denominators | `9/11` inline | similar digits: 1 → 7 | correct |
| 9 | 7 | `7/8 − 6/8 = [ ]` | 1 | fraction | `1/8` | `13/8` adds-instead | `1/8` stacked | correct | correct |
| 9 | 8 | `20 см — это [ ] метра` | 1 | fraction | `1/5` | `20/10` ten-centimetres | `1/6` inline | near key: 6 → 5 | not yet |
| 9 | 9 | `Сократи дробь: 15/18 = [ ]` | 1 | fraction in lowest terms | `5/6` | `15/18` not-in-lowest-terms, `5/18` reduces-numerator-only | `5/6` stacked | correct | correct |
| 9 | 10 | `1/12 + 9/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `5/6` | `10/12` not-in-lowest-terms, `10/24` adds-denominators | — | blank | unreadable, blank |
| 10 | 1 | `2 × [ ] = 6` | 1 | number | `3` | `4` wrong-operation | `3` | similar digits: 3 → 8 | correct |
| 10 | 2 | `6 : 2 = [ ]` | 1 | number | `3` | `4` table-neighbour | `3` | correct | correct |
| 10 | 3 | `[ ] + 15 = 61` | 1 | number | `46` | `76` adds-instead | `45` | near key: 5 → 6 | not yet |
| 10 | 4 | `670 − 282 = [ ]` | 1 | number | `388` | `412` no-borrow | `388` | correct | correct |
| 10 | 5 | `31 : 4 = [ ] (ост. [ ])` | 1 | number | `7` | `6` quotient-too-small | `6` | error | not yet, quotient-too-small |
| 10 | 5 | `31 : 4 = [ ] (ост. [ ])` | 2 | number | `3` | `7` remainder-too-big | `7` crossed out, `3` | crossed out | correct |
| 10 | 6 | `4/6 + 1/6 = [ ]` | 1 | fraction | `5/6` | `5/12` adds-denominators | `5/6` inline | mirrored digit: 5 | correct |
| 10 | 7 | `4/11 − 3/11 = [ ]` | 1 | fraction | `1/11` | `7/11` adds-instead | `1/11` stacked | correct | correct |
| 10 | 8 | `75 см — это [ ] метра` | 1 | fraction | `3/4` | `75/10` ten-centimetres | `3/4` inline | correct | correct |
| 10 | 9 | `Сократи дробь: 3/12 = [ ]` | 1 | fraction in lowest terms | `1/4` | `3/12` not-in-lowest-terms, `1/12` reduces-numerator-only | — | blank | unreadable, blank |
| 10 | 10 | `9/12 + 1/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `5/6` | `10/12` not-in-lowest-terms, `10/24` adds-denominators | `5/6` stacked | correct | correct |
| 11 | 1 | `4 × [ ] = 16` | 1 | number | `4` | `12` wrong-operation | `4` | correct | correct |
| 11 | 2 | `24 : 6 = [ ]` | 1 | number | `4` | `5` table-neighbour | `4` | correct | correct |
| 11 | 3 | `[ ] + 25 = 52` | 1 | number | `27` | `77` adds-instead | `27` | mirrored digit: 2 | correct |
| 11 | 4 | `735 − 494 = [ ]` | 1 | number | `241` | `361` no-borrow | `241` | correct | correct |
| 11 | 5 | `80 : 9 = [ ] (ост. [ ])` | 1 | number | `8` | `7` quotient-too-small | — | blank | unreadable, blank |
| 11 | 5 | `80 : 9 = [ ] (ост. [ ])` | 2 | number | `8` | `17` remainder-too-big | `8` | similar digits: 8 → 3 | correct |
| 11 | 6 | `5/7 + 1/7 = [ ]` | 1 | fraction | `6/7` | `6/14` adds-denominators | `6/7` inline | correct | correct |
| 11 | 7 | `9/11 − 1/11 = [ ]` | 1 | fraction | `8/11` | `10/11` adds-instead | `8/11` stacked | correct | correct |
| 11 | 8 | `15 минут — это [ ] часа` | 1 | fraction | `1/4` | `15/100` hundred-minutes | `15/100` inline | error | not yet, hundred-minutes |
| 11 | 9 | `Сократи дробь: 21/27 = [ ]` | 1 | fraction in lowest terms | `7/9` | `21/27` not-in-lowest-terms, `7/27` reduces-numerator-only | `7/4` stacked | near key: 4 → 9 | not yet |
| 11 | 10 | `3/6 + 1/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/3` | `4/6` not-in-lowest-terms, `4/12` adds-denominators | `4/6` inline crossed out, `2/3` inline | crossed out | correct |
| 12 | 1 | `9 × [ ] = 45` | 1 | number | `5` | `36` wrong-operation | `5` | similar digits: 5 → 6 | correct |
| 12 | 2 | `12 : 2 = [ ]` | 1 | number | `6` | `7` table-neighbour | `6` | correct | correct |
| 12 | 3 | `[ ] + 43 = 69` | 1 | number | `26` | `112` adds-instead | `26` | correct | correct |
| 12 | 4 | `624 − 378 = [ ]` | 1 | number | `246` | `354` no-borrow | `246` | correct | correct |
| 12 | 5 | `34 : 7 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | `4` | correct | correct |
| 12 | 5 | `34 : 7 = [ ] (ост. [ ])` | 2 | number | `6` | `13` remainder-too-big | `13` | error | not yet, remainder-too-big |
| 12 | 6 | `1/11 + 2/11 = [ ]` | 1 | fraction | `3/11` | `3/22` adds-denominators | `3/22` stacked crossed out, `3/11` stacked | crossed out | correct |
| 12 | 7 | `4/8 − 2/8 = [ ]` | 1 | fraction | `2/8` | `6/8` adds-instead | `2/8` inline | mirrored digit: 2 | correct |
| 12 | 8 | `10 минут — это [ ] часа` | 1 | fraction | `1/6` | `10/100` hundred-minutes | `1/5` stacked | near key: 5 → 6 | not yet |
| 12 | 9 | `Сократи дробь: 25/30 = [ ]` | 1 | fraction in lowest terms | `5/6` | `25/30` not-in-lowest-terms, `5/30` reduces-numerator-only | `5/6` inline | correct | correct |
| 12 | 10 | `7/10 + 1/10 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `4/5` | `8/10` not-in-lowest-terms, `8/20` adds-denominators | — | blank | unreadable, blank |
| 13 | 1 | `9 × [ ] = 81` | 1 | number | `9` | `72` wrong-operation | `9` | correct | correct |
| 13 | 2 | `63 : 9 = [ ]` | 1 | number | `7` | `8` table-neighbour | `7` | mirrored digit: 7 | correct |
| 13 | 3 | `[ ] + 24 = 52` | 1 | number | `28` | `76` adds-instead | `28` | correct | correct |
| 13 | 4 | `330 − 116 = [ ]` | 1 | number | `214` | `226` no-borrow | `226` crossed out, `214` | crossed out | correct |
| 13 | 5 | `38 : 7 = [ ] (ост. [ ])` | 1 | number | `5` | `4` quotient-too-small | `6` | near key: 6 → 5 | not yet |
| 13 | 5 | `38 : 7 = [ ] (ост. [ ])` | 2 | number | `3` | `10` remainder-too-big | `10` | error | not yet, remainder-too-big |
| 13 | 6 | `1/5 + 2/5 = [ ]` | 1 | fraction | `3/5` | `3/10` adds-denominators | `3/5` stacked | similar digits: 5 → 6 | correct |
| 13 | 7 | `8/11 − 1/11 = [ ]` | 1 | fraction | `7/11` | `9/11` adds-instead | `7/11` inline | correct | correct |
| 13 | 8 | `250 г — это [ ] кг` | 1 | fraction | `1/4` | `250/100` hundred-grams | `1/4` stacked | correct | correct |
| 13 | 9 | `Сократи дробь: 3/9 = [ ]` | 1 | fraction in lowest terms | `1/3` | `3/9` not-in-lowest-terms, `1/9` reduces-numerator-only | — | blank | unreadable, blank |
| 13 | 10 | `1/12 + 5/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/2` | `6/12` not-in-lowest-terms, `6/24` adds-denominators | `1/2` inline | correct | correct |
| 14 | 1 | `7 × [ ] = 49` | 1 | number | `7` | `42` wrong-operation | `42` | error | not yet, wrong-operation |
| 14 | 2 | `15 : 3 = [ ]` | 1 | number | `5` | `6` table-neighbour | `5` | mirrored digit: 5 | correct |
| 14 | 3 | `[ ] + 12 = 39` | 1 | number | `27` | `51` adds-instead | `21` | near key: 1 → 7 | not yet |
| 14 | 4 | `547 − 266 = [ ]` | 1 | number | `281` | `321` no-borrow | `281` | similar digits: 1 → 7 | correct |
| 14 | 5 | `34 : 8 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | `4` | correct | correct |
| 14 | 5 | `34 : 8 = [ ] (ост. [ ])` | 2 | number | `2` | `10` remainder-too-big | `2` | correct | correct |
| 14 | 6 | `6/9 + 2/9 = [ ]` | 1 | fraction | `8/9` | `8/18` adds-denominators | `8/18` stacked crossed out, `8/9` stacked | crossed out | correct |
| 14 | 7 | `4/5 − 1/5 = [ ]` | 1 | fraction | `3/5` | `5/5` adds-instead | — | blank | unreadable, blank |
| 14 | 8 | `5 см — это [ ] метра` | 1 | fraction | `1/20` | `5/10` ten-centimetres | `1/20` inline | correct | correct |
| 14 | 9 | `Сократи дробь: 5/20 = [ ]` | 1 | fraction in lowest terms | `1/4` | `5/20` not-in-lowest-terms, `1/20` reduces-numerator-only | `1/4` stacked | correct | correct |
| 14 | 10 | `1/6 + 2/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/2` | `3/6` not-in-lowest-terms, `3/12` adds-denominators | `1/2` inline | correct | correct |
| 15 | 1 | `5 × [ ] = 40` | 1 | number | `8` | `35` wrong-operation | `35` crossed out, `8` | crossed out | correct |
| 15 | 2 | `48 : 6 = [ ]` | 1 | number | `8` | `9` table-neighbour | `8` | correct | correct |
| 15 | 3 | `[ ] + 35 = 72` | 1 | number | `37` | `107` adds-instead | `31` | near key: 1 → 7 | not yet |
| 15 | 4 | `604 − 280 = [ ]` | 1 | number | `324` | `484` no-borrow | `484` | error | not yet, no-borrow |
| 15 | 5 | `35 : 4 = [ ] (ост. [ ])` | 1 | number | `8` | `7` quotient-too-small | `8` | correct | correct |
| 15 | 5 | `35 : 4 = [ ] (ост. [ ])` | 2 | number | `3` | `7` remainder-too-big | `3` | correct | correct |
| 15 | 6 | `4/10 + 3/10 = [ ]` | 1 | fraction | `7/10` | `7/20` adds-denominators | — | blank | unreadable, blank |
| 15 | 7 | `4/9 − 3/9 = [ ]` | 1 | fraction | `1/9` | `7/9` adds-instead | `1/9` stacked | mirrored digit: 9 | correct |
| 15 | 8 | `6 минут — это [ ] часа` | 1 | fraction | `1/10` | `6/100` hundred-minutes | `1/10` inline | similar digits: 1 → 7 | correct |
| 15 | 9 | `Сократи дробь: 3/6 = [ ]` | 1 | fraction in lowest terms | `1/2` | `3/6` not-in-lowest-terms, `1/6` reduces-numerator-only | `1/2` stacked | correct | correct |
| 15 | 10 | `1/8 + 5/8 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `3/4` | `6/8` not-in-lowest-terms, `6/16` adds-denominators | `3/4` inline | correct | correct |
| 16 | 1 | `7 × [ ] = 42` | 1 | number | `6` | `35` wrong-operation | — | blank | unreadable, blank |
| 16 | 2 | `12 : 3 = [ ]` | 1 | number | `4` | `5` table-neighbour | `4` | correct | correct |
| 16 | 3 | `[ ] + 32 = 62` | 1 | number | `30` | `94` adds-instead | `94` crossed out, `30` | crossed out | correct |
| 16 | 4 | `945 − 384 = [ ]` | 1 | number | `561` | `641` no-borrow | `567` | near key: 7 → 1 | not yet |
| 16 | 5 | `19 : 4 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | `4` | correct | correct |
| 16 | 5 | `19 : 4 = [ ] (ост. [ ])` | 2 | number | `3` | `7` remainder-too-big | `3` | correct | correct |
| 16 | 6 | `5/8 + 2/8 = [ ]` | 1 | fraction | `7/8` | `7/16` adds-denominators | `7/8` stacked | correct | correct |
| 16 | 7 | `3/8 − 2/8 = [ ]` | 1 | fraction | `1/8` | `5/8` adds-instead | `1/8` inline | correct | correct |
| 16 | 8 | `45 минут — это [ ] часа` | 1 | fraction | `3/4` | `45/100` hundred-minutes | `3/4` stacked | similar digits: 4 → 9 | correct |
| 16 | 9 | `Сократи дробь: 6/30 = [ ]` | 1 | fraction in lowest terms | `1/5` | `6/30` not-in-lowest-terms, `1/30` reduces-numerator-only | `6/30` inline | error | not yet, not-in-lowest-terms |
| 16 | 10 | `4/12 + 4/12 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/3` | `8/12` not-in-lowest-terms, `8/24` adds-denominators | `2/3` stacked | mirrored digit: 2 | correct |
| 17 | 1 | `9 × [ ] = 27` | 1 | number | `3` | `18` wrong-operation | `3` | correct | correct |
| 17 | 2 | `35 : 7 = [ ]` | 1 | number | `5` | `6` table-neighbour | `6` crossed out, `5` | crossed out | correct |
| 17 | 3 | `[ ] + 41 = 55` | 1 | number | `14` | `96` adds-instead | `14` | correct | correct |
| 17 | 4 | `551 − 228 = [ ]` | 1 | number | `323` | `337` no-borrow | `323` | correct | correct |
| 17 | 5 | `44 : 7 = [ ] (ост. [ ])` | 1 | number | `6` | `5` quotient-too-small | `6` | similar digits: 6 → 5 | correct |
| 17 | 5 | `44 : 7 = [ ] (ост. [ ])` | 2 | number | `2` | `9` remainder-too-big | — | blank | unreadable, blank |
| 17 | 6 | `3/6 + 2/6 = [ ]` | 1 | fraction | `5/6` | `5/12` adds-denominators | `5/6` inline | correct | correct |
| 17 | 7 | `3/6 − 2/6 = [ ]` | 1 | fraction | `1/6` | `5/6` adds-instead | `1/6` stacked | correct | correct |
| 17 | 8 | `24 минут — это [ ] часа` | 1 | fraction | `2/5` | `24/100` hundred-minutes | `2/5` inline | mirrored digit: 2 | correct |
| 17 | 9 | `Сократи дробь: 4/8 = [ ]` | 1 | fraction in lowest terms | `1/2` | `4/8` not-in-lowest-terms, `1/8` reduces-numerator-only | `7/2` stacked | near key: 7 → 1 | not yet |
| 17 | 10 | `4/9 + 2/9 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/3` | `6/9` not-in-lowest-terms, `6/18` adds-denominators | `6/9` inline | error | not yet, not-in-lowest-terms |
| 18 | 1 | `6 × [ ] = 24` | 1 | number | `4` | `18` wrong-operation | `4` | similar digits: 4 → 9 | correct |
| 18 | 2 | `18 : 3 = [ ]` | 1 | number | `6` | `7` table-neighbour | `6` | correct | correct |
| 18 | 3 | `[ ] + 37 = 80` | 1 | number | `43` | `117` adds-instead | `117` | error | not yet, adds-instead |
| 18 | 4 | `617 − 253 = [ ]` | 1 | number | `364` | `444` no-borrow | `364` | correct | correct |
| 18 | 5 | `22 : 6 = [ ] (ост. [ ])` | 1 | number | `3` | `2` quotient-too-small | `2` crossed out, `3` | crossed out | correct |
| 18 | 5 | `22 : 6 = [ ] (ост. [ ])` | 2 | number | `4` | `10` remainder-too-big | `9` | near key: 9 → 4 | not yet |
| 18 | 6 | `1/7 + 3/7 = [ ]` | 1 | fraction | `4/7` | `4/14` adds-denominators | `4/7` stacked | mirrored digit: 7 | correct |
| 18 | 7 | `7/11 − 2/11 = [ ]` | 1 | fraction | `5/11` | `9/11` adds-instead | `5/11` inline | correct | correct |
| 18 | 8 | `48 минут — это [ ] часа` | 1 | fraction | `4/5` | `48/100` hundred-minutes | `4/5` stacked | correct | correct |
| 18 | 9 | `Сократи дробь: 15/25 = [ ]` | 1 | fraction in lowest terms | `3/5` | `15/25` not-in-lowest-terms, `3/25` reduces-numerator-only | — | blank | unreadable, blank |
| 18 | 10 | `2/6 + 2/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `2/3` | `4/6` not-in-lowest-terms, `4/12` adds-denominators | `2/3` inline | correct | correct |
| 19 | 1 | `9 × [ ] = 72` | 1 | number | `8` | `63` wrong-operation | `8` | correct | correct |
| 19 | 2 | `40 : 5 = [ ]` | 1 | number | `8` | `9` table-neighbour | `9` crossed out, `8` | crossed out | correct |
| 19 | 3 | `[ ] + 36 = 81` | 1 | number | `45` | `117` adds-instead | `45` | mirrored digit: 5 | correct |
| 19 | 4 | `504 − 391 = [ ]` | 1 | number | `113` | `293` no-borrow | `118` | near key: 8 → 3 | not yet |
| 19 | 5 | `27 : 6 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | `3` | error | not yet, quotient-too-small |
| 19 | 5 | `27 : 6 = [ ] (ост. [ ])` | 2 | number | `3` | `9` remainder-too-big | `3` | correct | correct |
| 19 | 6 | `2/9 + 4/9 = [ ]` | 1 | fraction | `6/9` | `6/18` adds-denominators | — | blank | unreadable, blank |
| 19 | 7 | `3/11 − 2/11 = [ ]` | 1 | fraction | `1/11` | `5/11` adds-instead | `1/11` stacked | correct | correct |
| 19 | 8 | `500 г — это [ ] кг` | 1 | fraction | `1/2` | `500/100` hundred-grams | `1/2` inline | correct | correct |
| 19 | 9 | `Сократи дробь: 18/24 = [ ]` | 1 | fraction in lowest terms | `3/4` | `18/24` not-in-lowest-terms, `3/24` reduces-numerator-only | `3/4` stacked | correct | correct |
| 19 | 10 | `2/8 + 4/8 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `3/4` | `6/8` not-in-lowest-terms, `6/16` adds-denominators | `3/4` inline | similar digits: 4 → 9 | correct |
| 20 | 1 | `2 × [ ] = 14` | 1 | number | `7` | `12` wrong-operation | `7` | correct | correct |
| 20 | 2 | `8 : 2 = [ ]` | 1 | number | `4` | `5` table-neighbour | `4` | correct | correct |
| 20 | 3 | `[ ] + 17 = 39` | 1 | number | `22` | `56` adds-instead | `56` crossed out, `22` | crossed out | correct |
| 20 | 4 | `575 − 209 = [ ]` | 1 | number | `366` | `374` no-borrow | `366` | similar digits: 6 → 5 | correct |
| 20 | 5 | `17 : 4 = [ ] (ост. [ ])` | 1 | number | `4` | `3` quotient-too-small | — | blank | unreadable, blank |
| 20 | 5 | `17 : 4 = [ ] (ост. [ ])` | 2 | number | `1` | `5` remainder-too-big | `7` | near key: 7 → 1 | not yet |
| 20 | 6 | `1/7 + 4/7 = [ ]` | 1 | fraction | `5/7` | `5/14` adds-denominators | `5/7` stacked | mirrored digit: 5 | correct |
| 20 | 7 | `6/7 − 2/7 = [ ]` | 1 | fraction | `4/7` | `8/7` adds-instead | `4/7` inline | correct | correct |
| 20 | 8 | `10 см — это [ ] метра` | 1 | fraction | `1/10` | `10/10` ten-centimetres | `1/10` stacked | correct | correct |
| 20 | 9 | `Сократи дробь: 12/18 = [ ]` | 1 | fraction in lowest terms | `2/3` | `12/18` not-in-lowest-terms, `2/18` reduces-numerator-only | `2/3` inline | correct | correct |
| 20 | 10 | `2/6 + 1/6 = [ ] (несократимой дробью)` | 1 | fraction in lowest terms | `1/2` | `3/6` not-in-lowest-terms, `3/12` adds-denominators | `3/6` stacked | error | not yet, not-in-lowest-terms |
