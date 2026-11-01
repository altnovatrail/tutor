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

# What .github's shared release.yml runs to build a release, in the toolchain stage of the commit
# it releases, from that commit's tree as git archive writes it, and then signs every file (R62 in
# docs/decisions.md; R18 .github). The release is the Go module's tag, which api builds from, and
# its files are the source of that commit, tutor_<version>_source.tar.gz, and SHA256SUMS over it.
# dist/ is made afresh and holds plain files alone.
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
    rm -rf dist
    mkdir dist
    name="tutor_${version}"
    # The same tree gives the same bytes: names sorted, no owner, one time for every entry, the
    # one git archive gave each file, and no name or time in the gzip header. The spec submodule
    # is an empty folder in such a tree and stays out by name.
    tar --create --format=gnu --sort=name --owner=0 --group=0 --numeric-owner --mode=go-w \
        --mtime="@$(stat -c %Y justfile)" --exclude=./dist --exclude=./spec \
        --transform="s,^\.,${name},S" --file=- . | gzip -n -9 > "dist/${name}_source.tar.gz"
    (cd dist && sha256sum -- "${name}_source.tar.gz" > SHA256SUMS)

# Every file the code embeds with //go:embed is in the paths of release.yml, so that a change to it
# gives a release. Then release-files on a small tree of its own: it refuses a version without v, a
# folder with .git and a tree with a folder git ignores, writes two files with names .github's
# release takes and sums that check, leaves dist/ and spec/ at the top out of the archive but not a
# nested folder named spec, and gives the same bytes twice, a second apart.
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
    tree="$tmp/tree"
    mkdir -p "$tree/dist" "$tree/spec" "$tree/internal/spec"
    cp justfile LICENSE .gitignore "$tree/"
    echo old > "$tree/dist/old"
    echo kept > "$tree/internal/spec/kept.txt"
    run() { (cd "$tree" && just release-files "$1") > "$tmp/out" 2>&1; }
    ! run 0.1.0 || fail "it took a version without v"
    mkdir "$tree/.git"
    ! run v0.0.0 || fail "it ran in a folder with .git"
    rmdir "$tree/.git"
    for ignored in samples synthetic; do
        mkdir "$tree/$ignored"
        ! run v0.0.0 || fail "it ran in a tree with ${ignored}/, which git ignores"
        rmdir "$tree/$ignored"
    done
    run v0.0.0 || fail "it failed on a tree that git archive could write"
    archive="$tree/dist/tutor_v0.0.0_source.tar.gz"
    [ "$(ls "$tree/dist" | tr '\n' ' ')" = "SHA256SUMS tutor_v0.0.0_source.tar.gz " ] || fail "dist/ holds $(ls "$tree/dist" | tr '\n' ' ')"
    for file in "$tree"/dist/*; do
        [[ "$(basename "$file")" =~ ^[A-Za-z0-9_+][A-Za-z0-9._+-]*$ ]] || fail "$(basename "$file") is not a name release.yml takes"
    done
    (cd "$tree/dist" && sha256sum --quiet --check SHA256SUMS) || fail "SHA256SUMS does not check"
    entries="$(tar --list --gzip --file="$archive")"
    grep -qx 'tutor_v0.0.0/LICENSE' <<< "$entries" || fail "LICENSE is not in the archive"
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
# recipe, which is shell and not Go.
ci-test: release-files-test
    go test -race -count=1 ./...

# Fail on a known vulnerability in code this module reaches.
ci-vuln:
    go run {{ GOVULNCHECK }} ./...

# The list is ALLOWED_LICENSES, SPDX identifiers separated by commas, which .github's go.yml sets.
# Fail if a package this module is built from carries a license outside that list.
ci-licenses:
    @[ -n "${ALLOWED_LICENSES:-}" ] || { echo "ci-licenses: ALLOWED_LICENSES is empty or not set. CI takes it from .github's go.yml; to run this here, set it to the list there." >&2; exit 1; }
    go run {{ GO_LICENSES }} check ./... --allowed_licenses="$ALLOWED_LICENSES"
