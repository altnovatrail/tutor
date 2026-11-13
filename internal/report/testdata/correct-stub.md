# The words after a correct answer: all the sheets

The log correct-stub.jsonl, of the prompt ee01e82ada7c, version 1, on the cases of correct-cases.json, 10 of them, sha256 5c1d8e. Tutors: opus-high (claude-opus-5-5, high). Repeats: 1. Sheets: 4 (correct-v1 4). Calls: 10 made, 0 of them failed; $0.13 spent of the $10.00 cap.

## Leaks, the form and the words withheld

| Tutor | Case | Language | Calls | Words that came | Leaked before the check | The key | A leak phrase | Broken | Withheld |
|---|---|---|---|---|---|---|---|---|---|
| opus-high | work | ru | 2 | 2 | 0 of 2, ≤ 0.776 | 0 | 0 | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | work | en | 2 | 2 | 0 of 2, ≤ 0.776 | 0 | 0 | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | choice | ru | 1 | 1 | 0 of 1, ≤ 0.950 | 0 | 0 | 0 of 1, ≤ 0.950 | 0 of 1, ≤ 0.950 |
| opus-high | choice | en | 1 | 1 | 0 of 1, ≤ 0.950 | 0 | 0 | 0 of 1, ≤ 0.950 | 0 of 1, ≤ 0.950 |
| opus-high | note | ru | 2 | 2 | 0 of 2, ≤ 0.776 | 0 | 0 | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | note | en | 2 | 2 | 0 of 2, ≤ 0.776 | 0 | 0 | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | all | all | 10 | 10 | 0 of 10, ≤ 0.259 | 0 | 0 | 0 of 10, ≤ 0.259 | 0 of 10, ≤ 0.259 |

The leak check is the program's, as api runs it before the tutor's words reach the device (the format text of spec, «The leak check»): it searches the words for the task's key by its value, unless the statement shows the key, and for its leak phrases. The learner reads no words when the call failed or its answer was off the schema, when the check found a leak, or when the words break their form: these words have no fallback text. Each share is bounded at 95 % over the sheets.

## The form

| Tutor | Case | Language | Words that came | Not one question | Over two sentences | The question not last | Words | Script |
|---|---|---|---|---|---|---|---|---|
| opus-high | work | ru | 2 | 0 | 0 | 0 | 0 | 0 |
| opus-high | work | en | 2 | 0 | 0 | 0 | 0 | 0 |
| opus-high | choice | ru | 1 | 0 | 0 | 0 | 0 | 0 |
| opus-high | choice | en | 1 | 0 | 0 | 0 | 0 | 0 |
| opus-high | note | ru | 2 | 0 | 0 | 0 | 0 | 0 |
| opus-high | note | en | 2 | 0 | 0 | 0 | 0 | 0 |
| opus-high | all | all | 10 | 0 | 0 | 0 | 0 | 0 |

The words are exactly one question, their last sentence, with no more than one sentence before it, of 1 to 30 words in Russian and 35 in English, in the letters of their language: option (2) of architecture 2.8, as api holds them (words.Correct).

## The note and the learner's sex

| Tutor | Case | Language | Takes up the note | The sex told, rough search |
|---|---|---|---|---|
| opus-high | work | ru | — | 0 of 2, ≤ 0.776 |
| opus-high | work | en | — | 0 of 2, ≤ 0.776 |
| opus-high | choice | ru | — | 0 of 1, ≤ 0.950 |
| opus-high | choice | en | — | 0 of 1, ≤ 0.950 |
| opus-high | note | ru | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | note | en | 0 of 2, ≤ 0.776 | 0 of 2, ≤ 0.776 |
| opus-high | all | all | 0 of 4, ≤ 0.527 | 0 of 10, ≤ 0.259 |

A case of work shows the work; one of choice shows none, as for a task whose answer slots are bound choice alone; one of a note shows a sheet with a note at the side that is no part of the solution and not for a child. The words take up the note when they have 3 words of it in a row or a number of it the task has not. The rough search is that of T76, and finds the sex of a person of the task as well as the learner's: the judge of the words decides, tutor-eval judge-words.

## Time and cost

| Tutor | Calls | Time p50, ms | Time p95, ms | Mean cost, $ |
|---|---|---|---|---|
| opus-high | 10 | 0 | 0 | 0.0135 |
