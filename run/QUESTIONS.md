# QUESTIONS.md — questions to the author of `tutor`

Table 3 of [RUN.md](../RUN.md): the author's open questions that tasks leave (R153 spec, R154 spec, R160 spec).

- Never read whole: take the rows the task and its sections name — `grep -n '^| В1 ' run/QUESTIONS.md`.
- A row is a question a task leaves open (RUN.md rule 9). Record the author's answer where it belongs — the task, a document or [DEBT.md](DEBT.md) — and delete the row; [docs/plan/questions.md](../docs/plan/questions.md), «Закрыты», names where each answer went (R23).
- A row whose question opens with the mark «В реестр spec» is for `spec`: its session finds it on `main` (R81 spec; RULES.md, Across repositories). It leaves with the move to the tag of `spec` that recorded it: verbatim to «Переданы в spec» of [docs/plan/questions.md](../docs/plan/questions.md), and where the answer went to «Закрыты» there (R23).

## Table 3. Questions to the author

Numbers are not reused: next — В98.

| ID | Question | From | Options | Recommendation |
|---|---|---|---|---|
| В97 | «В реестр spec»: `spec`'s log cites blocks of architecture 3.4 tutor by their former Russian names — «Итог замера чтения», «Итог замера тетради», «Итог большого прогона», «Итог зеркальной цифры», «Итог замера подсказок» and others (`spec/docs/decisions.md`). The measurements left 3.4 for `docs/measurements.md` (R102), and the map of where each went is gone from 3.4 (R160 spec): «Итог замера свободной записи» — measurements 3.4 tutor, «Итог замера чтения» — 3.5, «Итог замера подсказок» and «Итог прогонов на листах автора» — 3.6, «Итог замера тетради» — 3.7, «Итог замера сочинений» — 3.8, «Выводы E4» — 3.9, «Итог новых конфигураций», «Итог двух вызовов сочинений» and «Итог десяти листов автора» — 3.11, «Итог большого прогона» — 3.12, «Итог зеркальной цифры» — 3.15 | T173 | (a) `spec` repoints each citation to its section of measurements tutor when a task touches the entry (R160 spec); (b) at once, in a task of its own; (c) `tutor` keeps the map in 3.4 | (a): the citations are history of the measurement, read rarely; a touched entry is rewritten anyway |
