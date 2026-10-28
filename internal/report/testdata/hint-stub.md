# The hint: the dev part

The log hint-stub.jsonl, of the prompt d8a2f994b155, version 1. Tutors: opus-high (claude-opus-5-5, high); haiku-low (claude-haiku-5-5, low); haiku-low-no-thinking (claude-haiku-5-5, low, without thinking). Repeats: 1. Rungs: 1, 2, 3. Languages: ru, en. Sheets with a mistake: 6 (lines 2, free 3, near 1). Calls: 108 made, 0 of them failed; $0.52 spent of the $10.00 cap.

## Leaks, before the check and after it, and the fallback texts

| Tutor | Language | Rung | Calls | Hints that came | Leaked before the check | The key | A leak phrase | Leaked after the check | Fallback | Failed | For a leak | For the form |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | en | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | ru | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | en | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | ru | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | en | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| opus-high | all | all | 36 | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 |
| haiku-low | ru | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | en | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | ru | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | en | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | ru | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | en | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low | all | all | 36 | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 |
| haiku-low-no-thinking | ru | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | en | 1 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | ru | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | en | 2 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | ru | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | en | 3 | 6 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 |
| haiku-low-no-thinking | all | all | 36 | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 |

The leak check is the program's, as api runs it before the tutor's words reach the device (the format text of spec, «The leak check»): it searches a hint for the task's key by its value, unless the statement shows the key, and for its leak phrases. A leak before the check is a hint that came in which it found either; after the check is what the learner reads — the hint, or the fallback text of the rung —, in which it finds nothing by construction. What the check does not search for, such as a number in words or the answer put another way, only a reader finds. The learner reads the fallback text when the call failed or its answer was off the schema, when the check found a leak, or when the hint breaks its form; a hint that leaks and breaks its form falls back for the leak. Each share is bounded at 95 % over the sheets.

## The form

| Tutor | Language | Rung | Hints that came | Broken | Not one question | Over two sentences | Words | Script | Broken by R70 spec |
|---|---|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | en | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | ru | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | en | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | ru | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | en | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| opus-high | all | all | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 |
| haiku-low | ru | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | en | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | ru | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | en | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | ru | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | en | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low | all | all | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 |
| haiku-low-no-thinking | ru | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | en | 1 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | ru | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | en | 2 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | ru | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | en | 3 | 6 | 0 of 6, ≤ 0.393 | 0 | 0 | 0 | 0 | 0 of 6, ≤ 0.393 |
| haiku-low-no-thinking | all | all | 36 | 0 of 36, ≤ 0.080 | 0 | 0 | 0 | 0 | 0 of 36, ≤ 0.080 |

A hint is exactly one question in no more than two sentences, of 1 to 30 words, in the letters of its language: Cyrillic in Russian, where a Latin letter the statement shows may stand too, and Latin in English (R39). R70 spec sets the same rules with 30 words in Russian and 35 in English, which the last column counts.

## The place and the trap

| Tutor | Language | Rung | Hints that came | Box on the first error | No box | The sheet's trap, on sheets with one | No trap, on sheets without |
|---|---|---|---|---|---|---|---|
| opus-high | ru | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | en | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | ru | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | en | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | ru | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | en | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| opus-high | all | all | 36 | 36 of 36, ≤ 1.000 | 0 | 30 of 30 | 6 of 6 |
| haiku-low | ru | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | en | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | ru | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | en | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | ru | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | en | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low | all | all | 36 | 36 of 36, ≤ 1.000 | 0 | 30 of 30 | 6 of 6 |
| haiku-low-no-thinking | ru | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | en | 1 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | ru | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | en | 2 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | ru | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | en | 3 | 6 | 6 of 6, ≤ 1.000 | 0 | 5 of 5 | 1 of 1 |
| haiku-low-no-thinking | all | all | 36 | 36 of 36, ≤ 1.000 | 0 | 30 of 30 | 6 of 6 |

The box is on the first error when it hits the part of the first error of the markup, as the judge's box does; the trap is the one the sheet was planned with, which the call did not get.

## Leaks by task

| Task | Its key searched for | Hints that came | Leaked | Found |
|---|---|---|---|---|
| chain-1 | yes | 18 | 0 |  |
| equation-3 | yes | 18 | 0 |  |
| album | yes | 18 | 0 |  |
| stamps | yes | 18 | 0 |  |
| grasshopper | yes | 18 | 0 |  |
| posts | no | 18 | 0 |  |

A key the statement of its task shows is not searched for, and its leak phrases alone keep the answer (the format text of spec, «The leak check»).

## Time and cost

| Tutor | Calls | Time p50, ms | Time p95, ms | Mean cost, $ |
|---|---|---|---|---|
| opus-high | 36 | 0 | 0 | 0.0137 |
| haiku-low | 36 | 0 | 0 | 0.0004 |
| haiku-low-no-thinking | 36 | 0 | 0 | 0.0003 |
