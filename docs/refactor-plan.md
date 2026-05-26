# Architecture Plan: openagentcontainers

> **Status: No structural refactoring required.**
> The package structure already satisfies Screaming Architecture and the Dependency Rule.
> This document formalises the existing boundaries and adds `go-arch-lint` enforcement.

---

## Section 1: Architecture Principles

**Screaming Architecture** requires that top-level package names reveal what a system _does_, not
how it is built. A reader who sees `companies/`, `jobs/`, `search/` immediately understands the
domain; a reader who sees `domain/`, `usecases/`, `repository/` learns only the layer pattern, not
the purpose. Every package in this project is named by capability: `oac` (manifest parsing), `check`
(validation), `discovery` (registry scanning), `dockerfile` (local file parsing), `search` (filtered
discovery), `scancache` (scan result caching).

**The Dependency Rule** requires that dependencies point inward: delivery layers import feature
layers, feature layers import nothing internal below them. Go's circular import prevention enforces
this at sub-package boundaries — a child package can import its parent, but not the reverse. In this
project the dependency graph flows strictly from `oac` (the foundation with no internal imports)
outward through `check`/`discovery`/`dockerfile`, then `search`, then the CLI delivery layer.

---

## Section 2: Module Paths

This project uses a Go workspace with two modules:

```
github.com/restrukt-ai/openagentcontainers/pkg  (defined in pkg/go.mod)
github.com/restrukt-ai/openagentcontainers/cli  (defined in cli/go.mod)
```

The `pkg` module is the **public library** — its packages are the stable API surface for external
consumers. The `cli` module is the **binary delivery layer** — it imports the library and adds CLI
plumbing. Cross-module dependencies (cli → pkg) are treated as external by each module's
go-arch-lint config.

---

## Section 3: Current Package Inventory

### `pkg` module — public library

| Package      | Responsibility                                                         | Key exports                                                 |
| ------------ | ---------------------------------------------------------------------- | ----------------------------------------------------------- |
| `oac`        | Parse OAC manifests from OCI image labels into versioned structs       | `Manifest`, `Image`, `Dockerfile`, `Parse()`, `SpecVersion` |
| `check`      | Validate parsed manifests; report errors and warnings                  | `Issue`, `Severity`, `Check()`                              |
| `discovery`  | Enumerate OCI registries; find OAC-conformant images                   | `Cache` interface, `Options`, `Discover()`, `FetchLabels()` |
| `dockerfile` | Parse Dockerfiles to extract LABEL pairs; decode into `oac.Dockerfile` | `ParseLabels()`, `Parse()`                                  |
| `search`     | Wrap `discovery.Discover` with case-insensitive substring filtering    | `Search()`                                                  |

### `cli` module — binary delivery

| Package                  | Responsibility                                                          | Key exports                        |
| ------------------------ | ----------------------------------------------------------------------- | ---------------------------------- |
| `cmd/oac` (main)         | Cobra CLI: `discover`, `search`, `check` subcommands; output formatting | `main()`                           |
| `cmd/internal/scancache` | File-backed, thread-safe JSON cache implementing `discovery.Cache`      | `Cache`, `Load()`, `DefaultPath()` |

---

## Section 4: Current Import Graph

```
pkg/oac
    (stdlib only — no internal deps)

pkg/check
    → pkg/oac

pkg/discovery
    → pkg/oac
    → github.com/google/go-containerregistry/pkg/crane
    → golang.org/x/time/rate

pkg/dockerfile
    → pkg/oac
    → github.com/moby/buildkit/frontend/dockerfile/parser

pkg/search
    → pkg/oac
    → pkg/discovery

cli/cmd/internal/scancache
    (stdlib only — no internal deps)

cli/cmd/oac
    → pkg/oac
    → pkg/check
    → pkg/discovery
    → pkg/dockerfile
    → pkg/search
    → cli/cmd/internal/scancache
    → github.com/spf13/cobra
    → github.com/google/go-containerregistry/pkg/crane
    → golang.org/x/time/rate
```

---

## Section 5: Identified Issues

**No structural issues were found.** The self-review checklist passes on every item:

- No layer-name packages (`domain/`, `usecases/`, `repository/`, `handlers/`, `services/`, `models/`)
- No domain types buried in `cmd/`; all are in `pkg/oac`
- Interfaces are defined where they are _used_: `discovery.Cache` lives in `discovery`, where `Discover` consumes it; the implementation (`scancache`) is correctly subordinate in `cli/cmd/internal/`
- Dependency direction is strictly inward: `oac` has no internal imports; nothing in `pkg/` imports `cli/`
- `pkg/` placement is intentional: these packages form the public library API for external consumers
- `scancache` is correctly CLI-private under `cmd/internal/`

**One gap: no architectural enforcement.** The correct structure is currently enforced only by
convention and code review. `go-arch-lint` is installed but not integrated into the lint pipeline.
Adding it creates a compiler-equivalent guard for the import graph.

---

## Section 6: Proposed Structure

The structure is correct as-is. Shown here for documentation:

```
pkg/                          # public library module
  oac/
    types.go                  # Manifest, Image, Dockerfile, specs, constants
    oac.go                    # Parse(), ParseSpecVersion(), custom UnmarshalJSON
  check/
    check.go                  # Check(), Issue, Severity
  discovery/
    discovery.go              # Discover(), FetchLabels(), Cache interface, Options
    retry.go                  # withRetry() helper
  dockerfile/
    dockerfile.go             # ParseLabels(), Parse()
  search/
    search.go                 # Search(), filterAgents()

cli/                          # binary delivery module
  cmd/
    oac/                      # main package — Cobra CLI
      main.go                 # discover + search commands + output formatting
      check.go                # check command + input mode detection
    internal/
      scancache/              # CLI-private; implements discovery.Cache
        cache.go
```

---

## Section 7: Dependency Direction

### `pkg` module

| Package      | May import (internal) | May import (external) |
| ------------ | --------------------- | --------------------- |
| `oac`        | (nothing)             | stdlib                |
| `check`      | `oac`                 | stdlib                |
| `discovery`  | `oac`                 | stdlib, crane, rate   |
| `dockerfile` | `oac`                 | stdlib, buildkit      |
| `search`     | `oac`, `discovery`    | stdlib                |

### `cli` module

| Package                  | May import (internal)    | May import (external / cross-module) |
| ------------------------ | ------------------------ | ------------------------------------ |
| `cmd/internal/scancache` | (nothing)                | stdlib                               |
| `cmd/oac`                | `cmd/internal/scancache` | stdlib, pkg/\*, cobra, crane, rate   |

Cross-module imports (`cmd/oac` → `pkg/*`) are tracked by the `cli` go.mod and workspace, not by go-arch-lint (which treats them as external).

---

## Section 8: Special Package Notes

**`search` as a thin wrapper.** `search.Search` is essentially `discovery.Discover` + a filter loop.
It exists to give external consumers a one-call API without requiring them to handle filtering
themselves. It is justified despite its size: it defines the filtering contract and absorbs future
evolution (pagination, scoring) without breaking callers.

**`scancache` location.** The `discovery.Cache` interface is defined in the `pkg/discovery` package
(where it is consumed), and its sole implementation lives in `cli/cmd/internal/scancache`. This is
correct Dependency Rule placement: the library defines the contract; the binary supplies the
concrete adapter. Any external consumer of `pkg/discovery` can supply their own `Cache`
implementation.

**Multi-module go-arch-lint.** Because `pkg` and `cli` are separate Go modules, go-arch-lint must
be run once per module from each module's root directory. Cross-module dependencies are external to
each run and are therefore not governed by the arch-lint configs. This is acceptable: the
workspace's go.mod / go.sum already enforce cross-module compatibility.

**`search` depending on `discovery`.** `search` imports `discovery` for `Options` and `Discover`.
This means `discovery`'s API surface is part of `search`'s interface too. This is intentional —
callers who want filtered discovery already need rate limiter and concurrency configuration, which
come from `discovery.Options`. If this coupling becomes a problem, `search` could define its own
options type; for now the shared type is cleaner.

---

## Section 9: go-arch-lint Configuration

These configs are written once during planning and are **frozen**. Violations during implementation
or maintenance are implementation problems, not config problems.

### `pkg/.go-arch-lint.yml`

```yaml
# THIS CONFIG IS FROZEN. DO NOT MODIFY TO SUPPRESS VIOLATIONS.
# Violations are implementation problems, not config problems.
# See docs/refactor-plan.md Section 9 for rationale.
#
# Notes:
# - vendor imports are unrestricted (go-arch-lint enforces internal boundaries only)
# - test files are excluded: external _test packages self-import the package under
#   test, which go-arch-lint otherwise flags as a spurious self-dependency
# - oac is listed in components but omitted from deps: it is the foundation package
#   with no internal imports; Go's circular-import prevention enforces this already,
#   and an empty mayDependOn array is invalid go-arch-lint syntax
version: 2
workdir: .
excludeFiles:
  - ".*_test\\.go$"
components:
  oac:
    in: oac
  check:
    in: check
  discovery:
    in: discovery
  dockerfile:
    in: dockerfile
  search:
    in: search
deps:
  check:
    anyVendorDeps: true
    mayDependOn:
      - oac
  discovery:
    anyVendorDeps: true
    mayDependOn:
      - oac
  dockerfile:
    anyVendorDeps: true
    mayDependOn:
      - oac
  search:
    anyVendorDeps: true
    mayDependOn:
      - oac
      - discovery
```

### `cli/.go-arch-lint.yml`

```yaml
# THIS CONFIG IS FROZEN. DO NOT MODIFY TO SUPPRESS VIOLATIONS.
# Violations are implementation problems, not config problems.
# See docs/refactor-plan.md Section 9 for rationale.
#
# Notes:
# - anyVendorDeps: true allows all external imports (pkg/* are cross-module,
#   treated as external/vendor from the cli module's perspective)
# - test files are excluded: external _test packages self-import the package under test
# - scancache is listed in components but omitted from deps: it has no internal
#   imports; Go's circular-import prevention already enforces this
version: 2
workdir: .
excludeFiles:
  - ".*_test\\.go$"
components:
  scancache:
    in: cmd/internal/scancache
  cli:
    in: cmd/oac
deps:
  cli:
    anyVendorDeps: true
    mayDependOn:
      - scancache
```

---

## Section 10: Migration Sequence

There is no structural migration. The one change to make is integrating go-arch-lint into the lint pipeline.

**Phase 1 — Write go-arch-lint configs and verify**

- [x] Write `pkg/.go-arch-lint.yml` (content above)
- [x] Write `cli/.go-arch-lint.yml` (content above)
- [x] Run `cd pkg && go-arch-lint check` — 0 violations
- [x] Run `cd cli && go-arch-lint check` — 0 violations
- [x] _Validate: `go build ./... && go test ./...` from workspace root_

**Phase 2 — Add go-arch-lint to Taskfiles**

- [x] Add `lint:go-arch-lint` task to `pkg/Taskfile.yml`
- [x] Add `lint:go-arch-lint` task to `cli/Taskfile.yml`
- [x] Wire each into the module `lint` task (alongside golangci-lint, govulncheck, nilaway)
- [x] Run `task lint` from workspace root — all green
- [x] _Validate: commit triggers pre-commit hook; hook runs lint; 0 failures_

---

## Section 11: File Mappings

### Moves and Merges

None. No files move.

### New Files

| Path                    | Content                                           |
| ----------------------- | ------------------------------------------------- |
| `pkg/.go-arch-lint.yml` | Arch-lint config for the `pkg` module (Section 9) |
| `cli/.go-arch-lint.yml` | Arch-lint config for the `cli` module (Section 9) |
| `docs/refactor-plan.md` | This document                                     |

---

## Section 12: New Code Required

| What                      | Where              | Description                                               |
| ------------------------- | ------------------ | --------------------------------------------------------- |
| `lint:go-arch-lint` task  | `pkg/Taskfile.yml` | `cd pkg && go-arch-lint --config .go-arch-lint.yml`       |
| `lint:go-arch-lint` task  | `cli/Taskfile.yml` | `cd cli && go-arch-lint --config .go-arch-lint.yml`       |
| Dependency in `lint` task | Both Taskfiles     | Add `go-arch-lint` as a step in the `lint` aggregate task |

---

## Section 13: Interface Contract Changes

No interfaces change. `discovery.Cache` remains in `pkg/discovery`; `scancache.Cache` in `cli/cmd/internal/scancache` continues to implement it. No moves, no renames.

---

## Section 14: Success Criteria

The architecture formalisation is complete when:

- [x] `go build ./...` passes with no errors (from workspace root)
- [x] `go test ./...` passes with no failures (from workspace root)
- [x] `cd pkg && go-arch-lint --config .go-arch-lint.yml` passes with zero violations
- [x] `cd cli && go-arch-lint --config .go-arch-lint.yml` passes with zero violations
- [x] go-arch-lint runs as part of `task lint` in both modules
- [x] No package named after a layer (`domain`, `usecases`, `repository`, `handlers`, `services`, `models`) exists anywhere in the project

The go-arch-lint configs must not be modified after this planning phase. Any future violation is an implementation problem: either a new import was added that violates the architecture, or a new package was created without updating this plan.

---

## Subagent Delegation Pattern

Each phase above is small enough to be implemented in a single session. When delegating:

- Give the subagent this plan document and tell it which phase to implement
- Require worktree isolation (`isolation: "worktree"`)
- The go-arch-lint configs in Section 9 are frozen — the subagent must not modify them
- If go-arch-lint reports violations the subagent cannot resolve without changing the config, it must stop and report to the project lead
- The subagent must run `go build ./...`, `go test ./...`, and both `go-arch-lint` invocations before reporting done
