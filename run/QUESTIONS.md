# QUESTIONS.md — questions to the author of `tutor`

Table 3 of [RUN.md](../RUN.md): the author's open questions that tasks leave (R153 spec, R160 spec).

- Never read whole: take the rows the task and its sections name — `grep -n '^| В1 ' run/QUESTIONS.md`.
- A row is a question a task leaves open (RUN.md rule 9). Record the author's answer where it belongs — the task, a document or [DEBT.md](DEBT.md) — and delete the row; [docs/plan/questions.md](../docs/plan/questions.md), «Закрыты», names where each answer went (R23).
- A row whose question opens with the mark «В реестр spec» is for `spec`: its session finds it on `main` (R81 spec; RULES.md, Across repositories). It leaves with the move to the tag of `spec` that recorded it: verbatim to «Переданы в spec» of [docs/plan/questions.md](../docs/plan/questions.md), and where the answer went to «Закрыты» there (R23).

## Table 3. Questions to the author

Numbers are not reused: next — В99.

| ID | Question | From | Options | Recommendation |
|---|---|---|---|---|
| В98 | «В реестр spec»: under `"citations": true` the copies of three of spec's leak vectors in `internal/leak/testdata/leaks/` fail `docs-check-repo`: their comments cite sections — `numbers-in-a-picture-path.yaml` — `(section 20)`, `superscripts.yaml` — `as in section 15`, `rubric.yaml` — `(architecture 3.6)`. In spec they are `format/` vectors, whose comments the rule does not read; in tutor the checker reads them, and `TestCopies` holds every copy byte for byte to `spec/format/vectors/leaks/`, so tutor cannot reword them. T195 turns the key on for the whole repository. Решено исполнителем по рекомендации, до проверки автора: (a); the copies stay as they are until a tag of spec brings the reworded vectors | T197 | (a) spec rewords the three comments in words — a comment of a vector changes no contract — and tutor copies them with its next move to a tag; (b) spec's component profile skips the copies of spec's `format/` vectors a repository keeps, named in `docs-budgets.json`; (c) tutor alone: `TestCopies` compares the vectors without their comments, and tutor rewords its copies | (a): the smallest change, the copies stay spec's byte for byte, and every repository that copies the vectors gains it |
