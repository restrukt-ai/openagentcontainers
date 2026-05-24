# Open Agent Containers

A standard for packaging AI agents as containers that declare their own runtime requirements.

## The Standard

An AI agent needs more than a model and a prompt at runtime: a live inference endpoint,
credentials for the external services it calls, filesystem storage, and a channel back to the
orchestrator that launched it. Baking this configuration into the image couples it to a specific
deployment. Passing it through ad-hoc environment variables gives the orchestrator no structured
way to know what the agent needs before it runs.

OAC inverts this. The agent image declares its requirements as Docker labels. Any compliant
orchestrator reads those labels and satisfies them — no out-of-band config files, no per-agent
knowledge hardcoded into the infrastructure.

```dockerfile
FROM node:25-alpine3.22

LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="my-agent"

# inference
LABEL org.openagentcontainers.inference.provider="openai"
LABEL org.openagentcontainers.inference.model="gpt-4o"

# orchestrator connection
LABEL org.openagentcontainers.orchestrator.url="https://orchestrator.example.com"
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_URL"
```

All labels are namespaced under `org.openagentcontainers`. The six label groups:

| Group           | Declares                                                        |
| --------------- | --------------------------------------------------------------- |
| `inference`     | Required inference provider and model                           |
| `mcp`           | MCP server URLs and credential negotiation (bearer, OAuth, DCR) |
| `workspaces`    | Filesystem volume mounts the agent needs                        |
| `orchestrator`  | Orchestrator endpoint and authentication (bearer or mTLS)       |
| `events`        | Application-level event channel subscriptions and schemas       |
| `session`       | Per-session runtime isolation settings                          |

The harness inside the container must initiate an outbound ConnectRPC bidirectional stream to the
orchestrator at startup using the address injected via the env var declared in
`orchestrator.env`. Establishing the stream signals readiness — no separate registration message
is required.

## Go Library (`pkg/`)

```
github.com/restrukt-ai/openagentcontainers
```

Three packages:

**`oac`** — types, parsing, and validation. `Parse(labels)` converts a flat label map into a
typed, versioned `Manifest`. `Manifest.Validate()` enforces required fields.

**`discovery`** — concurrent OCI registry scanner. `Discover(ctx, registry, opts)` catalogs a
registry and returns all OAC-conformant images. Supports configurable concurrency, rate limiting,
retries, and a digest-keyed cache to avoid re-fetching previously seen images.

**`search`** — wraps discovery with text filtering. `Search(ctx, registry, query, opts)` returns
images whose name, version, description, or any label value contains the query string
(case-insensitive).

## CLI

The `oac` CLI provides discovery and search over OCI registries.

```sh
# discover all OAC-conformant images in a registry
oac discover registry.example.com

# search by name, description, or any label value
oac search registry.example.com "weather"

# output as JSON
oac discover registry.example.com --json
```

Key flags (shared by both commands):

| Flag              | Default | Description                                      |
| ----------------- | ------- | ------------------------------------------------ |
| `-c, --concurrency` | `10`  | Concurrent workers                               |
| `-r, --rate-limit`  | `10`  | Max registry requests/sec (0 = unlimited)        |
| `--max-retries`     | `3`   | Retries on transient errors before giving up     |
| `--json`            |       | Output as JSON instead of a table                |
| `--insecure`        |       | Use HTTP instead of HTTPS                        |
| `--force`           |       | Scan all tags even if `latest` lacks OAC labels  |
| `--no-cache`        |       | Disable the local scan cache                     |
| `--cache-path`      |       | Override the cache file location                 |

The scan cache lives at `~/.cache/oac/registry.json`. Entries are keyed by OCI digest
(content-addressed, never expire) so repeated scans of large registries are fast.

## Spec & Docs

The full label reference, conformance requirements, and spec rationale are in
[`SPEC.md`](./SPEC.md) and at the [spec website](https://openagentcontainers.dev). The spec
site source is in [`spec-site/`](./spec-site).
