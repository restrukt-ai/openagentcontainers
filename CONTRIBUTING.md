# Contributing

## Prerequisites

| Tool                                                                | Purpose                                                  |
| ------------------------------------------------------------------- | -------------------------------------------------------- |
| Go 1.26+                                                            | Build and test the library and CLI                       |
| [Task](https://taskfile.dev)                                        | Run dev commands (`task`)                                |
| [lefthook](https://github.com/evilmartians/lefthook)                | Git hooks                                                |
| [commitlint](https://commitlint.js.org)                             | Commit message validation — install globally (see Setup) |
| [uv](https://docs.astral.sh/uv/)                                    | Spec site (MkDocs) — only needed if editing the site     |
| [golangci-lint](https://golangci-lint.run)                          | Go linting                                               |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Dependency vulnerability scanning                        |
| [nilaway](https://github.com/uber-go/nilaway)                       | Nil safety analysis                                      |

## Setup

```bash
npm install -g @commitlint/cli @commitlint/config-conventional

git clone https://github.com/restrukt-ai/openagentcontainers.git
cd openagentcontainers
lefthook install     # installs git hooks
```

## Repository layout

```
SPEC.md              # canonical specification — source of truth
pkg/                 # Go library (github.com/restrukt-ai/openagentcontainers/pkg)
cli/                 # oac CLI — consumes pkg
spec-site/           # MkDocs site that publishes the spec
  docs/
    index.md         # overview page
    reference.md     # label reference
    changelog.md     # changelog
  mkdocs.yml
```

`pkg/` and `cli/` are separate Go modules. Changes that affect the public API of `pkg/` require a
corresponding update in `cli/`'s `go.mod` once the new version is available.

## Common tasks

```bash
task lint     # golangci-lint + govulncheck + nilaway across pkg/ and cli/
task test     # go test -race ./... across pkg/ and cli/
task tidy     # go mod tidy across pkg/ and cli/
```

Spec site:

```bash
task spec:serve   # live-reload dev server at localhost:8889
task spec:build   # build static site to spec-site/dist/
```

## Git hooks

The pre-commit hook runs `task tidy`, `task lint`, and `task test` automatically before every
commit. The commit-msg hook validates your commit message against the conventional commits format
(see below).

If a hook fails, fix the issue, re-stage, and commit again. Never use `--no-verify`.

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/). The commit-msg hook
enforces this.

```
<type>: <short description>

feat: add mcp.oauth.dcr label support
fix: correct env var injection order at startup
docs: clarify orchestrator connection requirements
chore: update golangci-lint to v2
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`.

Breaking changes: add `!` after the type (`feat!:`) and include a `BREAKING CHANGE:` footer.

## Working on the spec

`SPEC.md` is the authoritative source. The spec site (`spec-site/`) publishes it —
`docs/reference.md` and `docs/index.md` must stay consistent with `SPEC.md`.

When changing the spec:

- Update `SPEC.md` first.
- Update `spec-site/docs/reference.md` if any labels were added, removed, or changed.
- Add an entry to `spec-site/docs/changelog.md`.
- Update the Go types in `pkg/` if the label schema changed.

Minor clarifications (wording, examples) don't need a changelog entry. Anything that changes what a
compliant implementation must do does.

## Working on the Go code

`pkg/` is the library importers depend on. Keep its public API stable within a version. Prefer
adding over changing; if a breaking change is necessary, note it in the commit message and changelog.

`cli/` is the `oac` binary. It is a consumer of `pkg/` and should not expose any logic that belongs
in the library.

Lint is configured to be strict. If golangci-lint reports an issue, fix the root cause — do not
add `//nolint` directives without a comment explaining why suppression is correct.

## Submitting changes

| Change type | Issue first?                                                                                              |
| ----------- | --------------------------------------------------------------------------------------------------------- |
| Spec        | Yes, if non-trivial — spec changes affect all implementors, so it's worth aligning before writing.        |
| Code        | Not required for bug fixes and small improvements. For larger additions, an issue helps establish intent. |
| Docs only   | No — just open a PR.                                                                                      |

Each PR should do one thing.
