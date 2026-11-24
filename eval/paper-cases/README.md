# Cases of a task from paper, exported

Each `*.json` file here is a Check that the debug mode exported: the crop of the task, of the work and of the instruction above the frame as the models saw them, what each call returned, the Check's verdict and what it read, and the truth as the developer set it — the value read and the verdict of each task. The developer saves a file with **Save case** in the debug window of `app` and moves it here — only a Check of the developer's own paper: a file holds the pictures of the work of the profile it was made in, and no learner's work goes into git.

Run the models on them:

```sh
tutor-eval run -kind statement -part dev -paper eval/paper-cases
tutor-eval run -kind check -part dev -paper eval/paper-cases
```

Every case is of the part `dev`; a file's name sets its order. The statement's run asks only the cases whose every task has the statement Recognise read. The pictures are the developer's own paper, never a learner's work.
