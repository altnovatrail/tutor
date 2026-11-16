# QUESTIONS.md — questions to the author of `tutor`

Table 3 of [RUN.md](../RUN.md): the author's open questions from tasks' «Открыто» (R153 spec, R154 spec).

- Never read whole: take the rows the task and its sections name — `grep -n '^| В85 ' run/QUESTIONS.md`.
- A row comes from a task's «Открыто» (RUN.md rule 9). Record the author's answer where it belongs — the task, a document or [DEBT.md](DEBT.md) — and delete the row; [docs/plan/questions.md](../docs/plan/questions.md), «Закрыты», names where each answer went (R23).
- A row whose question opens with the mark «В реестр spec» is for `spec`: its session finds it on `main` (R81 spec; RULES.md, Across repositories). It leaves with the move to the tag of `spec` that recorded it: verbatim to «Переданы в spec» of [docs/plan/questions.md](../docs/plan/questions.md), and where the answer went to «Закрыты» there (R23, the 2026-11-06 amendment).

## Table 3. Questions to the author

Numbers are not reused: next — В86.

| ID | Question | From | Options | Recommendation |
|---|---|---|---|---|
| В85 | «В реестр spec»: `just new-repo-draft tutor go 3.6 v0.18.4` writes into a copy of the kind `go` the comment of `spec`'s own stage `toolchain`, "spec's tests need nothing the core lacks, so here the stage adds nothing", which in a copy names the wrong repository's tests; `tutor` rewrote it by hand (R121), and every copy of the kinds `go` and `typescript` gets the same sentence | T150, `.devcontainer/Dockerfile` | (a) `new-repo` writes a sentence for a copy — what the kind adds, or that it adds nothing; (b) leave it, each move to a tag rewrites the comment | (a): one template text instead of a hand edit on every move, and `new-repo-check` can then hold it |
