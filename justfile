# tutor's own recipes: build, test and lint its Go code in the devcontainer, and start the board,
# the check board or its record mode. The ci- ones are what .github's shared go.yml runs, one job
# each, in the toolchain stage of .devcontainer/Dockerfile, from a checkout without the spec
# submodule (R16 .github; R13 in docs/decisions.md): their names are the same in every Go
# repository of NovaTrail, and what they run is this repository's own.
# The documents are checked by spec's recipe: just -f spec/justfile docs-check-repo

set shell := ["bash", "-euo", "pipefail", "-c"]

# The tools only the checks need, at exact versions. Each is a Go program run straight from its
# module, as module-kit's justfile runs them, so nothing is installed for it: the Go module proxy
# serves that version and the checksum database vouches for it. Dependabot does not see them,
# so they are raised by hand.
GOVULNCHECK := "golang.org/x/vuln/cmd/govulncheck@v1.9.0"
GO_LICENSES := "github.com/google/go-licenses/v2@v2.0.1"

# Build every package, the eval runner among them.
build:
    go build ./...

# Run every test.
test:
    go test ./...

# Run the linters and check the formatting; nothing is changed.
lint:
    golangci-lint run ./...

# Format the code in place, the way lint checks it.
fmt:
    golangci-lint fmt ./...

# This recipe asks for each eval key that is not set yet, without showing it, so that no key lands
# in the shell's history; Enter skips that model. Its arguments go to tutor-eval board.
# Start the check board on http://127.0.0.1:8080 (R19 in docs/decisions.md).
board *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${TUTOR_ANTHROPIC_API_KEY:-}" ]; then
        read -rsp 'Anthropic key for eval, or Enter to skip: ' TUTOR_ANTHROPIC_API_KEY
        echo >&2
    fi
    if [ -z "${TUTOR_OPENAI_API_KEY:-}" ]; then
        read -rsp 'OpenAI key for eval, or Enter to skip: ' TUTOR_OPENAI_API_KEY
        echo >&2
    fi
    export TUTOR_ANTHROPIC_API_KEY TUTOR_OPENAI_API_KEY
    exec go run ./cmd/tutor-eval board {{ args }}

# This recipe asks for no key: the record mode asks no model. The sheets go to samples/sheets,
# out of git. Its arguments go to tutor-eval board.
# Start the record mode of the board on http://127.0.0.1:8080 (R25 in docs/decisions.md).
record *args:
    go run ./cmd/tutor-eval board -record {{ args }}

# This recipe asks for no key either. The sheets of the author's free-form plan, three sheets of
# parts, go to samples/free, out of git. Its arguments go to tutor-eval board.
# Start the record mode on the free-form sheets of epic E4 (T61 in RUN.md).
free-record *args:
    go run ./cmd/tutor-eval board -record -dir samples/free -bank eval/free-bank.json -plan eval/free-author.json {{ args }}

# This recipe asks for the Anthropic key for eval if it is not set yet, without showing it, so that
# no key lands in the shell's history; with -stub the run needs none, and Enter skips it. Its
# arguments go to tutor-eval run: -part dev or -part test, and the rest.
# Run the measurement on a part of the split, after its estimate and your yes (T23 in RUN.md).
measure *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${TUTOR_ANTHROPIC_API_KEY:-}" ]; then
        read -rsp 'Anthropic key for eval, or Enter with -stub: ' TUTOR_ANTHROPIC_API_KEY
        echo >&2
    fi
    export TUTOR_ANTHROPIC_API_KEY
    exec go run ./cmd/tutor-eval run {{ args }}

# This recipe is measure on the 40 sheets that code draws for free writing, in synthetic/free, out
# of git, and it asks for the key as measure does; the run logs in synthetic/runs. Its arguments go
# to tutor-eval run: -part dev, -part test or -part all, and the rest.
# Run the measurement of free writing on the sheets of code, after its estimate and your yes (T65 in RUN.md).
free *args:
    just measure -dir synthetic/free -bank eval/free-bank.json -plan eval/free-sheets.json {{ args }}

# This recipe asks for the Anthropic key for eval if it is not set yet, as measure does; with -stub
# the runs need none. It runs the judge, version 2, and the reading — without a key, with the
# right and the false keys, at the other heights of digits and on the states — over all the
# author's three sheets of free writing in samples/free, each after its estimate and your yes, and
# prints each report on the terminal. The logs and the reports go to samples/runs, out of git, and
# the reports hold no handwriting. Each run is capped at $1, so that both stay within the $2 of
# T77. Its arguments go to both runs, such as -stub or another -max-usd. A run you say no to, or one
# that stops short, does not keep the other from running, and the report is of the logs there are.
# Run the judge and the reading on the author's three sheets (T77 in RUN.md).
live *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${TUTOR_ANTHROPIC_API_KEY:-}" ]; then
        read -rsp 'Anthropic key for eval, or Enter with -stub: ' TUTOR_ANTHROPIC_API_KEY
        echo >&2
    fi
    export TUTOR_ANTHROPIC_API_KEY
    stamp="$(date -u +%Y%m%d-%H%M%S)"
    judge="samples/runs/${stamp}-live-judge.jsonl"
    reading="samples/runs/${stamp}-live-reading.jsonl"
    status=0
    go run ./cmd/tutor-eval run -part all -max-usd 1 -dir samples/free -bank eval/free-bank.json -plan eval/free-author.json -log "$judge" {{ args }} || status=1
    go run ./cmd/tutor-eval run -kind reading -part all -max-usd 1 -reads keys,heights,states -set samples/free,eval/free-bank.json,eval/free-author.json -log "$reading" {{ args }} || status=1
    for log in "$judge" "$reading"; do
        if [ -f "$log" ]; then
            go run ./cmd/tutor-eval report "$log" || status=1
            if [ -f "${log%.jsonl}.md" ]; then cat "${log%.jsonl}.md"; fi
        fi
    done
    exit "$status"

# Run lint, the linters and the formatting check, under the name go.yml runs.
ci-lint: lint

# Run every test, with the race detector and without the test cache.
ci-test:
    go test -race -count=1 ./...

# Fail on a known vulnerability in code this module reaches.
ci-vuln:
    go run {{ GOVULNCHECK }} ./...

# The list is ALLOWED_LICENSES, SPDX identifiers separated by commas, which .github's go.yml sets.
# Fail if a package this module is built from carries a license outside that list.
ci-licenses:
    @[ -n "${ALLOWED_LICENSES:-}" ] || { echo "ci-licenses: ALLOWED_LICENSES is empty or not set. CI takes it from .github's go.yml; to run this here, set it to the list there." >&2; exit 1; }
    go run {{ GO_LICENSES }} check ./... --allowed_licenses="$ALLOWED_LICENSES"
