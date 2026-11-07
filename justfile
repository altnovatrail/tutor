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

# This recipe asks for no key either. The author's ten sheets of the extended measurement, solved in
# free form and written messily on purpose, go to samples/free-messy, out of git. Its arguments go
# to tutor-eval board.
# Start the record mode on the ten messy sheets of the extended measurement (T97 in RUN.md).
messy-record *args:
    go run ./cmd/tutor-eval board -record -dir samples/free-messy -bank eval/free-bank.json -plan eval/free-messy.json {{ args }}

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

# The file of the open views of the measurement's tasks, which tutor's release carries for canvas's
# sample page: what the learner sees of each task of the banks, the workbook and the essays, and
# none of its key, solution, traps or checks (T37; R64 in docs/decisions.md). A change to one of
# those files fails the tests until this recipe writes the file anew. The new file takes the old
# one's place only once it is written whole.
# Write eval/open-tasks.json anew from the files of the measurement's tasks.
open-tasks:
    #!/usr/bin/env bash
    set -euo pipefail
    new="$(mktemp -u eval/open-tasks.XXXXXX.json)"
    trap 'rm -f "$new"' EXIT
    go run ./cmd/tutor-eval open -file "$new" eval/bank.json eval/free-bank.json eval/workbook.json eval/essays.json
    mv "$new" eval/open-tasks.json

# What .github's shared release.yml runs to build a release, in the toolchain stage of the commit
# it releases, from that commit's tree as git archive writes it, and then signs every file (R62 in
# docs/decisions.md; R18 .github). The release is the Go module's tag, which api builds from, and
# its files are the source of that commit, tutor_<version>_source.tar.gz; the open views of the
# measurement's tasks, tutor_<version>_open-tasks.json, a copy of eval/open-tasks.json, which
# canvas's CI puts into its sample page (R64); and SHA256SUMS over both. dist/ is made afresh and
# holds plain files alone.
# Build a release's files into dist/, for a version such as v0.1.0.
release-files version:
    #!/usr/bin/env bash
    set -euo pipefail
    version={{ quote(version) }}
    if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
        echo "release-files: the version is v<major>.<minor>.<patch>, as v0.1.0, not ${version}" >&2
        exit 1
    fi
    # Only a tree that git archive wrote holds a commit's files and nothing else. A working copy
    # also holds what git does not track, samples/ among it, the author's handwriting, which never
    # leaves it (CLAUDE.md). So the recipe refuses a folder with .git, and one that holds a folder
    # .gitignore names at the top, dist/ aside: no commit's tree has one.
    if [ -e .git ]; then
        echo "release-files: this folder has .git; run it in a tree that git archive wrote, as release.yml does: git archive HEAD | tar -x -C <folder>" >&2
        exit 1
    fi
    while IFS= read -r line; do
        folder="${line#/}"
        folder="${folder%/}"
        if [[ "$line" =~ ^/[^/*?]+/$ ]] && [ "$folder" != dist ] && [ -e "$folder" ]; then
            echo "release-files: ${folder}/ is here, and git ignores it, so this is no tree that git archive wrote" >&2
            exit 1
        fi
    done < .gitignore
    if [ ! -f eval/open-tasks.json ]; then
        echo "release-files: eval/open-tasks.json, the open views a release carries, is not here" >&2
        exit 1
    fi
    rm -rf dist
    mkdir dist
    name="tutor_${version}"
    # The same tree gives the same bytes: names sorted, no owner, one time for every entry, the
    # one git archive gave each file, and no name or time in the gzip header. The spec submodule
    # is an empty folder in such a tree and stays out by name.
    tar --create --format=gnu --sort=name --owner=0 --group=0 --numeric-owner --mode=go-w \
        --mtime="@$(stat -c %Y justfile)" --exclude=./dist --exclude=./spec \
        --transform="s,^\.,${name},S" --file=- . | gzip -n -9 > "dist/${name}_source.tar.gz"
    cp eval/open-tasks.json "dist/${name}_open-tasks.json"
    (cd dist && sha256sum -- "${name}_open-tasks.json" "${name}_source.tar.gz" > SHA256SUMS)

# Every file the code embeds with //go:embed is in the paths of release.yml, and so is the file of
# open views, so that a change to it gives a release. Then release-files on a small tree of its
# own: it refuses, each for its reason, a version without v, a folder with .git, a tree with a folder
# git ignores and one without the file of open views, leaving dist/ as it was, writes three files with names .github's release takes and sums
# that check, the file of open views as it is, leaves dist/ and spec/ at the top out of the archive
# but not a nested folder named spec, and gives the same bytes twice, a second apart.
# Check release.yml's paths against what the code embeds, and release-files in a temporary folder.
release-files-test:
    #!/usr/bin/env bash
    set -euo pipefail
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    fail() {
        echo "release-files-test: $*" >&2
        if [ -s "$tmp/out" ]; then
            echo "release-files said last:" >&2
            cat "$tmp/out" >&2
        fi
        exit 1
    }
    while IFS=: read -r file directive; do
        read -ra patterns <<< "${directive#//go:embed }"
        for pattern in "${patterns[@]}"; do
            embedded="$(dirname "${file#./}")/${pattern#all:}"
            grep -qxF "        ${embedded}" .github/workflows/release.yml || fail "${embedded}, which ${file#./} embeds, is not in the paths of .github/workflows/release.yml"
        done
    done < <(grep -r --include='*.go' --exclude='*_test.go' --exclude-dir=spec --exclude-dir=testdata '^//go:embed ' . || true)
    grep -qxF "        eval/open-tasks.json" .github/workflows/release.yml || fail "eval/open-tasks.json, which a release carries, is not in the paths of .github/workflows/release.yml"
    tree="$tmp/tree"
    mkdir -p "$tree/dist" "$tree/spec" "$tree/internal/spec" "$tree/eval"
    cp justfile LICENSE .gitignore "$tree/"
    cp eval/open-tasks.json "$tree/eval/"
    echo old > "$tree/dist/old"
    echo kept > "$tree/internal/spec/kept.txt"
    run() { (cd "$tree" && just release-files "$1") > "$tmp/out" 2>&1; }
    # A refusal counts only for its own reason: the run fails, and says why.
    refused() { ! run "$1" && grep -qF -- "$2" "$tmp/out"; }
    refused 0.1.0 "the version is v<major>" || fail "it took a version without v"
    mkdir "$tree/.git"
    refused v0.0.0 "this folder has .git" || fail "it ran in a folder with .git"
    rmdir "$tree/.git"
    for ignored in samples synthetic; do
        mkdir "$tree/$ignored"
        refused v0.0.0 "${ignored}/ is here, and git ignores it" || fail "it ran in a tree with ${ignored}/, which git ignores"
        rmdir "$tree/$ignored"
    done
    mv "$tree/eval/open-tasks.json" "$tmp/open-tasks.json"
    refused v0.0.0 "eval/open-tasks.json, the open views a release carries, is not here" || fail "it ran in a tree without eval/open-tasks.json"
    [ -f "$tree/dist/old" ] || fail "it emptied dist/ before it refused"
    mv "$tmp/open-tasks.json" "$tree/eval/"
    run v0.0.0 || fail "it failed on a tree that git archive could write"
    archive="$tree/dist/tutor_v0.0.0_source.tar.gz"
    [ "$(ls "$tree/dist" | tr '\n' ' ')" = "SHA256SUMS tutor_v0.0.0_open-tasks.json tutor_v0.0.0_source.tar.gz " ] || fail "dist/ holds $(ls "$tree/dist" | tr '\n' ' ')"
    cmp -s "$tree/dist/tutor_v0.0.0_open-tasks.json" eval/open-tasks.json || fail "the file of open views in dist/ is not eval/open-tasks.json"
    for file in "$tree"/dist/*; do
        [[ "$(basename "$file")" =~ ^[A-Za-z0-9_+][A-Za-z0-9._+-]*$ ]] || fail "$(basename "$file") is not a name release.yml takes"
    done
    (cd "$tree/dist" && sha256sum --quiet --check SHA256SUMS) || fail "SHA256SUMS does not check"
    [ "$(wc -l < "$tree/dist/SHA256SUMS")" -eq 2 ] || fail "SHA256SUMS does not sum both files"
    entries="$(tar --list --gzip --file="$archive")"
    grep -qx 'tutor_v0.0.0/LICENSE' <<< "$entries" || fail "LICENSE is not in the archive"
    grep -qx 'tutor_v0.0.0/eval/open-tasks.json' <<< "$entries" || fail "eval/open-tasks.json is not in the archive"
    grep -qx 'tutor_v0.0.0/internal/spec/kept.txt' <<< "$entries" || fail "a nested folder named spec is not in the archive"
    ! grep -qv '^tutor_v0.0.0/' <<< "$entries" || fail "an entry is outside tutor_v0.0.0/"
    ! grep -qE '^tutor_v0.0.0/(dist|spec)(/|$)' <<< "$entries" || fail "dist/ or spec/ is in the archive"
    first="$(sha256sum < "$archive")"
    sleep 1
    run v0.0.0 || fail "it failed the second time"
    [ "$(sha256sum < "$archive")" = "$first" ] || fail "the same tree gave other bytes"
    echo "release-files-test: ok"

# Run lint, the linters and the formatting check, under the name go.yml runs.
ci-lint: lint

# Run every test, with the race detector and without the test cache, and the check of the release
# recipe, which is shell and not Go. The coverage of the tests goes to coverage/go.out, one Go
# profile, which the coverage job of .github's shared go.yml sends to Codecov (R48 spec; R66). A
# package's lines count by its own tests: -coverpkg=./... would make the profile 25 times larger
# and the run twice as long for 1.4 points more (R66).
ci-test: release-files-test
    mkdir -p coverage
    go test -race -count=1 -covermode=atomic -coverprofile=coverage/go.out ./...

# The drawn samples of spec's eval protocol that the tests hold to what tutor-eval draw writes,
# one of each kind it draws (internal/evalsample's testdata), checked against the rules of sections
# 11 and 12 of spec's text of the contracts by spec's own recipe, with the buf of the toolchain stage
# (R69). go test -update in internal/evalsample writes them anew.
# Check the drawn samples of testdata with spec's sample-check.
sample-check:
    just -f spec/justfile sample-check internal/evalsample/testdata/*.json

# Run sample-check under the name the repository's own job Samples in ci.yml runs; it needs the
# spec submodule, which go.yml does not check out.
ci-sample-check: sample-check

# Fail on a known vulnerability in code this module reaches.
ci-vuln:
    go run {{ GOVULNCHECK }} ./...

# The list is ALLOWED_LICENSES, SPDX identifiers separated by commas, which .github's go.yml sets.
# Fail if a package this module is built from carries a license outside that list.
ci-licenses:
    @[ -n "${ALLOWED_LICENSES:-}" ] || { echo "ci-licenses: ALLOWED_LICENSES is empty or not set. CI takes it from .github's go.yml; to run this here, set it to the list there." >&2; exit 1; }
    go run {{ GO_LICENSES }} check ./... --allowed_licenses="$ALLOWED_LICENSES"
