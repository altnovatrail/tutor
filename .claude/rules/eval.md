---
paths:
  - "cmd/tutor-eval/**"
  - "eval/**"
  - "justfile"
  - "internal/board/**"
  - "internal/convert/**"
  - "internal/essay/**"
  - "internal/evalsample/**"
  - "internal/gen/**"
  - "internal/judge/**"
  - "internal/measure/**"
  - "internal/report/**"
  - "internal/runner/**"
  - "internal/sample/**"
  - "internal/samplecheck/**"
  - "internal/sheets/**"
  - "internal/synth/**"
  - "internal/workbook/**"
  - "internal/wordsjudge/**"
---

# The eval runner and the runs

`tutor-eval` draws the sets, runs eval on them through the library's own code, and reports; the board and its record mode are part of it (R19, R25). Where each command writes and reads, beyond what CLAUDE.md says of `samples/` and `synthetic/`:

- The board and its record mode write only to `samples/` and refuse any other folder in the work tree, `synthetic/` among them. `draw`, `sheets`, `split` and `run` take a folder in either, but a run on sheets in `samples/` logs there alone (R05, R28).
- `draw -workbook` and `draw -essays` write the samples of the workbook's pages and of the essays, which code draws, and with `-png` their crops, in `synthetic/` alone or outside the work tree, and `workbook`, `essays` and their runs read the samples there (R40, R44, R71).
- `convert` writes the sheets of tutor-sample v0 of a directory as samples of the protocol beside it, in the directory's name and `-v1`, and those of `samples/` only in `samples/`; it and the record mode keep the author's writer's code in `samples/writer-code` (R75).
- `canvas` reads the synthetic samples of a release of canvas, which canvas's code draws, from `synthetic/canvas-<tag>`, with the trees of `eval/` at the tags of tutor they name in `synthetic/tutor-trees`, and logs in `synthetic/runs`; `just canvas-fetch` writes both (R76).
- The set of the tutor's words is code's too: `draw` draws its sheets, `eval/words-sheets.json`, into `synthetic/words-v1`, and `judge-words` logs beside the log of the hint it judges, or the judge's control phrases in `synthetic/runs` (R85).

**Defaults.** The board, `draw`, `sheets`, `split` and `run`, and the recipes `just board`, `just record`, `just measure`, `just free-record`, `just messy-record` and `just admit-record`, have their default directories in `samples/`, but for the pages of the workbook and the essays, which `draw -workbook`, `draw -essays`, `workbook`, `essays` and their runs find in `synthetic/` and the name of their file, `synthetic/workbook` (R71); `just free` gives `run` the sheets of code in `synthetic/free-v1`, and `just live` runs the judge and the reading on the author's three sheets in `samples/free-v1` and logs in `samples/runs` (T77). The directories of sheets are those of samples of the protocol, `-v1`, since T105; the files of tutor-sample v0 beside them only `convert` reads (R75). A command that falls back to one writes or reads among the author's sheets. So give each such command its `-dir`, and `-log` where it takes one: in `synthetic/` for the commands of the sheets drawn by code, and outside the work tree for a test of the board.

**Paid runs.** A paid run on `synthetic/` the executor starts itself, within the ceiling its task names: that ceiling is the author's confirmation under rule 4 of RUN.md (R28). The run also keeps under the spending limit the author set for the eval key's workspace at the provider, and the task's «Сделано» names what was spent: the provider, the number of calls and the sum (R146 spec; R110). The day's prices are checked on Anthropic's Pricing page before each run. The key for eval stays outside the repository, in `~/.config/tutor/anthropic.key`: the run gets it in its own environment, `TUTOR_ANTHROPIC_API_KEY="$(cat ~/.config/tutor/anthropic.key)"`, and no command prints it.

**Checks.** `just sample-check` holds the drawn samples of `testdata/` to spec's `sample-check` (R69), and `just canvas-check` and `just canvas-check-test` that the runner reads the samples of the pinned release of canvas and fails on a spoiled one (R76); CI runs them as `ci-sample-check` and `ci-canvas-check`. A change to a file of the measurement's tasks fails the tests until `just open-tasks` writes `eval/open-tasks.json` anew (R64). The Go checks are the go rule, [go.md](go.md).
