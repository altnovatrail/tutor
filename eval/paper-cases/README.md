# Cases of a task from paper, exported

Each `*.json` file here is a Check that the debug mode of a local run exported: the crop of the task and of the work as the models saw them, what each call returned, and the truth as the developer set it — the formula, the key, the value read and the verdict (R136). The developer saves a file with **Save case** in the debug window of `app` and moves it here.

Run the models on them:

```sh
tutor-eval run -kind statement -part dev -paper eval/paper-cases
tutor-eval run -kind solving -part dev -paper eval/paper-cases
tutor-eval run -kind paper -part dev -paper eval/paper-cases
```

Every case is of the part `dev`; a file's name sets its order. The pictures are the developer's own paper, never a learner's work.
