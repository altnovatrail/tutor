# The hint: all the sheets

The log words-stub.jsonl, of the prompt 7d598021bf71, version 3. Tutors: opus-high (claude-opus-5-5, high). Repeats: 1. Rungs: 1, 2, 3. Languages: ru, en. Sheets with a mistake: 4 (words-v1 4). Calls: 30 made, 0 of them failed; $0.45 spent of the $10.00 cap.

## Leaks, before the check and after it, and the fallback texts

| Tutor | Language | Rung | Calls | Hints that came | Leaked before the check | The key | A leak phrase | Leaked after the check | Fallback | Failed | For a leak | For the form |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | en | 1 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | ru | 2 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | en | 2 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | ru | 3 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | en | 3 | 5 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 |
| opus-high | all | all | 30 | 30 | 0 of 30, ≤ 0.095 | 0 | 0 | 0 | 0 of 30, ≤ 0.095 | 0 | 0 | 0 |

The leak check is the program's, as api runs it before the tutor's words reach the device (the format text of spec, «The leak check»): it searches a hint for the task's key by its value, unless the statement shows the key, and for its leak phrases. A leak before the check is a hint that came in which it found either; after the check is what the learner reads — the hint, or the fallback text of the rung —, in which it finds nothing by construction. What the check does not search for, such as a number in words or the answer put another way, only a reader finds. The learner reads the fallback text when the call failed or its answer was off the schema, when the check found a leak, or when the hint breaks its form; a hint that leaks and breaks its form falls back for the leak. Each share is bounded at 95 % over the sheets.

## The form

| Tutor | Language | Rung | Hints that came | Broken | Not one question | Over two sentences | Words | Script | Broken by R70 spec |
|---|---|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | en | 1 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | ru | 2 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | en | 2 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | ru | 3 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | en | 3 | 5 | 0 of 5, ≤ 0.451 | 0 | 0 | 0 | 0 | 0 of 5, ≤ 0.451 |
| opus-high | all | all | 30 | 0 of 30, ≤ 0.095 | 0 | 0 | 0 | 0 | 0 of 30, ≤ 0.095 |

A hint is exactly one question in no more than two sentences, of 1 to 30 words in Russian and 35 in English, in the letters of its language: Cyrillic in Russian, where a Latin letter the statement shows may stand too, and Latin in English. These are the rules of R70 spec, which version 2 of the prompt is held to as api holds hints (words.Hint), so the last column counts the same as Broken.

## The place and the trap

| Tutor | Language | Rung | Hints that came | Box on the first error | No box | The sheet's trap, on sheets with one | No trap, on sheets without |
|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 5 | 4 of 4, ≤ 1.000 | 0 | 1 of 1 | 0 of 0 |
| opus-high | en | 1 | 5 | 4 of 4, ≤ 1.000 | 0 | 2 of 2 | 0 of 0 |
| opus-high | ru | 2 | 5 | 4 of 4, ≤ 1.000 | 0 | 1 of 1 | 0 of 0 |
| opus-high | en | 2 | 5 | 4 of 4, ≤ 1.000 | 0 | 2 of 2 | 0 of 0 |
| opus-high | ru | 3 | 5 | 4 of 4, ≤ 1.000 | 0 | 1 of 1 | 0 of 0 |
| opus-high | en | 3 | 5 | 4 of 4, ≤ 1.000 | 0 | 2 of 2 | 0 of 0 |
| opus-high | all | all | 30 | 24 of 24, ≤ 1.000 | 0 | 9 of 9 | 0 of 0 |

The box is on the first error when it hits the part of the first error of the markup, as the judge's box does; the trap is the one the sheet was planned with, which the call did not get. Of the set of the words, a call without the work counts in neither column of the box, and a call given the trap the program found counts in neither column of the trap.

## The set of the words

The cases of words-cases.json, 30 of them, sha256 9df2b1a2a1f6 (T118).

| Tutor | Case | Language | Calls | Hints that came | Leaked | Broken | Repeats a question asked | A box without the work | Takes up the note | The sex told, rough search |
|---|---|---|---|---|---|---|---|---|---|---|
| opus-high | rung | ru | 6 | 6 | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 | 0 of 4, ≤ 0.527 | — | — | 0 of 6, ≤ 0.393 |
| opus-high | rung | en | 6 | 6 | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 | 0 of 4, ≤ 0.527 | — | — | 0 of 6, ≤ 0.393 |
| opus-high | otherwise | ru | 3 | 3 | 0 of 3, ≤ 0.632 | 0 of 3, ≤ 0.632 | 0 of 3, ≤ 0.632 | 0 of 3 | — | 0 of 3, ≤ 0.632 |
| opus-high | otherwise | en | 3 | 3 | 0 of 3, ≤ 0.632 | 0 of 3, ≤ 0.632 | 0 of 3, ≤ 0.632 | 0 of 3 | — | 0 of 3, ≤ 0.632 |
| opus-high | note | ru | 6 | 6 | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 | 0 of 4, ≤ 0.527 | — | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 |
| opus-high | note | en | 6 | 6 | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 | 0 of 4, ≤ 0.527 | — | 0 of 6, ≤ 0.393 | 0 of 6, ≤ 0.393 |
| opus-high | all | all | 30 | 30 | 0 of 30, ≤ 0.095 | 0 of 30, ≤ 0.095 | 0 of 22, ≤ 0.127 | 0 of 6 | 0 of 12, ≤ 0.221 | 0 of 30, ≤ 0.095 |

A case of the rung shows the work and gives the questions of the rungs below; one of the rung put another way shows none and gives the question of the rung itself too; one of a note shows a sheet with a note at the side that is no part of the solution and not for a child. A hint repeats a question when it holds 80 % of its words; it takes up the note when it has 3 words of it in a row or a number of it the task has not. The rough search is that of T76, and finds the sex of a person of the task as well as the learner's: the judge of the words decides, tutor-eval judge-words.

## Leaks by task

| Task | Its key searched for | Hints that came | Leaked | Found |
|---|---|---|---|---|
| lines-gen-1-001 | yes | 18 | 0 |  |
| lines-gen-1-010 | yes | 6 | 0 |  |
| lines-gen-1-005 | yes | 6 | 0 |  |

A key the statement of its task shows is not searched for, and its leak phrases alone keep the answer (the format text of spec, «The leak check»).

## Time and cost

| Tutor | Calls | Time p50, ms | Time p95, ms | Mean cost, $ |
|---|---|---|---|---|
| opus-high | 30 | 0 | 0 | 0.0149 |
