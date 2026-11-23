# tutor

[![CI](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/ci.yml)
[![Release](https://github.com/altnovatrail/tutor/actions/workflows/release.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/release.yml)
[![CodeQL](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml/badge.svg)](https://github.com/altnovatrail/tutor/actions/workflows/codeql.yml)
[![Latest release](https://img.shields.io/github/v/release/altnovatrail/tutor)](https://github.com/altnovatrail/tutor/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/altnovatrail/tutor)](go.mod)
[![Coverage](https://codecov.io/gh/altnovatrail/tutor/branch/main/graph/badge.svg)](https://codecov.io/gh/altnovatrail/tutor)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/16171/badge)](https://www.bestpractices.dev/projects/16171)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/altnovatrail/tutor/badge)](https://scorecard.dev/viewer/?uri=github.com/altnovatrail/tutor)
[![License](https://img.shields.io/github/license/altnovatrail/tutor)](LICENSE)

[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=code_smells)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=altnovatrail_tutor&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=altnovatrail_tutor)

The AI side of the NovaTrail tutor. It reads the learner's work, handwriting or text, into an observation, reviews the working for the first mistake, assesses an essay against the module author's rubric, and words the tutor's replies for the rung of the hint ladder that the platform's program chose. It holds the platform's prompts, each with its own eval set, the adapters to AI providers, the models admitted for each capability and the one function that admits an AI route for the learner's age category; its eval runner works on the same code.

It decides no verdict, picks no lesson or task and stores nothing, and its requests carry no identifier of the learner or the account: the model sees the work, never whose it is.

A component of [NovaTrail](https://github.com/altnovatrail/spec), a universal open platform for adaptive learning, interactive lessons and AI tutoring. Its place in the platform:

- **api** calls it as a library, in its own process, and reaches AI providers only through it; api picks the rung of the hint ladder, counts the platform AI quotas by the usage tutor reports for each call and checks the tutor's words for a leak before they reach the device.
- **module-kit** gives the verdict on the observation tutor reads, and tutor's eval runner counts false alarms by verdict through it; the CI of **modules** runs that runner on each module's eval set.
- **engine** picks what comes next, without a model.
- **canvas** is where the learner writes the work that tutor reads; tutor's CI checks that its eval runner reads canvas's synthetic samples, and canvas's samples page takes the open views of tutor's eval tasks from tutor's release.

Its card is in [the architecture of spec](https://github.com/altnovatrail/spec/blob/main/docs/architecture.md#36-tutor--the-tutor-reading-and-words); the detail, the decisions and everything else are in [docs/](docs/README.md).
