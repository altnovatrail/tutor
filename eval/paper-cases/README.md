# Cases of a task from paper, exported

Each `*.json` file here is a Check that the debug mode exported: the crop of the task and of the work as the models saw them, what each call returned, and the truth as the developer set it — the formula, the key, the value read and the verdict (R136). The developer saves a file with **Save case** in the debug window of `app` and moves it here — only a Check of the developer's own paper: a file holds the pictures of the work of the profile it was made in, and no learner's work goes into git.

Run the models on them:

```sh
tutor-eval run -kind statement -part dev -paper eval/paper-cases
tutor-eval run -kind solving -part dev -paper eval/paper-cases
tutor-eval run -kind paper -part dev -paper eval/paper-cases
```

Every case is of the part `dev`; a file's name sets its order. The pictures are the developer's own paper, never a learner's work.
