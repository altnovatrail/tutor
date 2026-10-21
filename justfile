# tutor's own recipes: build, test and lint its Go code in the devcontainer. The ci- ones are what
# .github's shared go.yml runs, one job each, in the toolchain stage of .devcontainer/Dockerfile,
# from a checkout without the spec submodule (R16 .github; R13 in docs/decisions.md): their names
# are the same in every Go repository of NovaTrail, and what they run is this repository's own.
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
