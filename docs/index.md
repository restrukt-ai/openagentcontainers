# Open Agent Containers

Every AI agent deployment requires the same manual work: wire up an inference endpoint, inject
credentials, mount storage. **Open Agent Containers** (OAC) eliminate that work. An OAC is a Docker image that declares its own runtime requirements as OCI
labels — a compliant orchestrator reads those labels and provisions everything automatically,
in any environment, without per-agent configuration outside the image.

## The Problem: Agents Need Infrastructure

An AI agent is more than a model and a prompt. At runtime it needs a live LLM endpoint to call,
credentials for the external services it uses, filesystem storage to operate on, and a channel
back to the orchestrator that launched it. These are infrastructure concerns — and how they are
satisfied changes between environments (local dev, staging, production, multi-tenant SaaS).

The naive solution is to bake this configuration into the image itself or pass it through
ad-hoc environment variables documented in a README. Both approaches break portability: the image
is coupled to a specific deployment, and the orchestrator has no structured way to know what the
agent actually needs before it runs.

## The Solution: Self-Describing Artifacts

OAC inverts the model. The agent image declares its own requirements as Docker labels. The
orchestrator reads those labels and satisfies them — without any out-of-band configuration files
or per-agent knowledge hardcoded into the infrastructure.

```
┌─────────────────────────────────────────────────────────┐
│  Docker image                                           │
│                                                         │
│  runtime harness + prompt + tools (inside the image)    │
│                                                         │
│  labels: what I need from the outside world ──────────────────► orchestrator
│    - inference endpoint                                 │
│    - OAuth credentials for MCP servers                  │
│    - filesystem mounts                                  │
│    - orchestrator connection + auth                     │
│    - event subscriptions                                │
└─────────────────────────────────────────────────────────┘
```

The image is a complete, portable artifact. The labels are its contract with the infrastructure.
A compliant orchestrator in any environment can read that contract and fulfill it.

## The Two Sides of the Contract

**The artifact author** (the person writing the Dockerfile) declares:

- Which inference capabilities and models the agent requires
- Which MCP servers need OAuth credentials, and what auth methods are acceptable
- What filesystem workspaces the agent needs and whether they must be writable
- How the harness connects to the orchestrator and how that connection is authenticated
- Which application-level event channels the agent subscribes to, and their schemas

**The orchestrator** reads those declarations and decides how to satisfy them:

- Which inference gateway to inject, after validating declared model availability
- Which registered OAuth client or IAT to use for each MCP server
- Which volumes to mount and where
- What token or certificate to issue the harness for its outbound stream
- How to route and transform inbound events before delivering them

Neither side needs to know the other's internals. The author doesn't know (or care) which Ollama
instance the orchestrator manages. The orchestrator doesn't know (or care) how the harness
implements its chat loop.

## Label Namespace

All labels are namespaced under `org.openagentcontainers`. A docker image declares itself as an
OAC-compliant container by including its agent name and spec version. See the
[label reference](reference.md) for all available labels.

```dockerfile
FROM node:25-alpine3.22

LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="pi-weather"
```

Labels can be inspected without pulling the image:

```bash
# local image
docker image inspect pi-weather --format '{{json .Config.Labels}}' | jq

# remote image
docker manifest inspect ghcr.io/org/pi-weather:latest
```

## Lifecycle

Agent requirements flow through three distinct phases:

```
Build time          Registration time        Startup time
──────────          ─────────────────        ────────────
Author writes   →   Orchestrator reads   →   Orchestrator injects
labels into         image labels and         credentials, mounts
Dockerfile          caches schemas           volumes, env vars
```

**Build time** — Labels are written into the Dockerfile. They are static declarations: they
describe what the agent needs, not where to find it. Infrastructure-specific URLs and secrets
never appear in the image.

**Registration time** — When the image is registered with the orchestrator (not at every cold
start), the orchestrator inspects the labels and, for agents with event subscriptions, extracts
the declared schema files from the image. It caches everything it needs to act on an inbound
event before the container is running.

**Startup time** — When the orchestrator launches the container, it performs Dynamic Client
Registration for any MCP servers that declare DCR, provisions workspace volumes, and injects
credentials and addresses into the container via environment variables or mounted secret files —
exactly as declared in the labels.

## Runtime Requirements

OAC does not prescribe the internal implementation of the agent harness — the process that drives
the LLM loop, manages MCP connections, and handles inputs can be written in any language using any
framework. However, there are concrete interface requirements the harness must fulfill to
participate in the OAC ecosystem.

**Orchestrator connection** — The harness must initiate an outbound ConnectRPC bidirectional
stream to the orchestrator at startup, using the address injected via the env var declared in
`orchestrator.env`. If the image declares an auth method (`orchestrator.bearer.*` or
`orchestrator.mtls.*`), the harness must use the injected credentials to authenticate the stream.
Establishing the stream is the signal that the harness is ready to receive events — no separate
registration message is required. ConnectRPC supports the Connect, gRPC, and gRPC-Web protocols;
compliant harnesses must speak at least one.

**Event schema files** — If the image declares event subscriptions, each schema file must be
present in the image at its declared path at build time. The orchestrator extracts these files
from the image without running the container; schema files must not be generated at runtime.

## Cloud-Native Fit

OAC is designed to ride on top of existing cloud-native infrastructure, not replace it. An agent
container is just a workload — it participates in existing registry workflows (access control, tag
immutability, vulnerability scanning, replication), runs under existing container orchestration,
and emits to existing observability pipelines.

A compliant OAC image can be stored in any OCI-compatible registry (Harbor, ECR, GHCR, Docker
Hub) and scheduled by any orchestrator that reads its labels. The goal is to make agents
first-class infrastructure primitives within the ecosystem platform teams already operate —
not to introduce a parallel stack they have to manage separately.

The natural deployment target for a compliant orchestrator is Kubernetes, where a custom operator
and CRDs let platform teams declare desired agent state and have it reconciled like any other
workload.

## Why Labels, Not a Config File

Docker labels are part of the OCI image spec and are universally readable: by container runtimes,
registries, CI pipelines, and orchestrators. They survive image pushes, pulls, and re-tagging.
Any tool that can inspect an image can read an agent's requirements — no SDK, no sidecar, no
separate config endpoint.

A structured JSON or YAML config file inside the image would require running the container (or at
least extracting and parsing a file) to discover requirements. Labels are available from the image
manifest alone, making inspection cheap and enabling registration-time schema caching without
ever starting the container.

## What the Spec Does Not Cover

The following are left to the runtime harness inside the image:

- **Prompt / system prompt**
- **Skills and tools**
- **MCP server configuration** — labels only cover credential negotiation, not how servers are launched or connected.
- **Event payload format** — the spec declares schemas for orchestrator-side transformation; wire format is an implementation concern.
