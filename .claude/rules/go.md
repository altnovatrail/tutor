---
paths:
  - "**/*.go"
  - "go.mod"
  - "go.sum"
  - ".golangci.yml"
---

# The Go code of tutor

`just build` builds every package, the eval runner among them; `just test` runs every test; `just lint` runs golangci-lint — the linters and the formatting check of `.golangci.yml` — and changes nothing; `just fmt` formats in place. A task that touches Go code ends with `just lint` and `just ci-test` green.

CI runs the same code by four recipes, `ci-lint`, `ci-test`, `ci-vuln` and `ci-licenses`, which `.github`'s shared `go.yml` runs as "Go / lint", "Go / test", "Go / vuln" and "Go / licenses", in the `toolchain` stage, from a checkout without the `spec` submodule. `ci-test` runs `go test -race`, with a timeout of 15 minutes, on the packages `RACE` of the justfile names, those whose code or tests start goroutines — the root, `internal/board`, `provider/claude` and `provider/gpt` —, and the rest without the detector, which finds a race only where goroutines run and costs CI its time elsewhere. `TestRaceStartsGoroutines`, in `tutor_test.go`, holds `RACE` to the code both ways: a package that comes to start a goroutine fails it until it is in `RACE`. `cmd/tutor-eval` runs its longest tests in parallel: run `ci-test` alone, not beside another heavy check. It writes `coverage/go.out`, one profile of both runs, which "Go / coverage" sends to Codecov, and a merge waits for `codecov/patch`, at least 90% of the changed lines covered. A new known vulnerability of the standard library fails "Go / vuln" with no change to the code: Go is raised then by the ci rule, [ci.md](ci.md).

Tests that hold what crosses the parts:
- `TestSpecVersion`, in `internal/openview`, holds the `spec` submodule, spec's Go modules in `go.mod` — `spec` and `spec/contracts` — and the tag CLAUDE.md names to one tag: a move to a tag moves them together, with `module-kit`;
- `TestRootImportsNoProvider`, in `tutor_test.go`, keeps the root package free of the providers' adapters and their SDKs: a service takes the adapters it needs itself;
- the golden logs under `internal/report/testdata/` name the version of `module-kit`, so a move of it rewrites them.

Tests run on synthetic strokes that the code draws, never on a sample of `samples/`, and they call a provider only through a stub. A run that costs money goes by the rule of the eval runner, [eval.md](eval.md).

A new dependency carries a license of spec's list, which `ci-licenses` checks.
