# tutor's own recipes: build, test and lint its Go code in the devcontainer, and in CI once its
# Go checks run there (В14 in RUN.md). The documents are checked by spec's recipe:
# just -f spec/justfile docs-check-repo

set shell := ["bash", "-euo", "pipefail", "-c"]

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
