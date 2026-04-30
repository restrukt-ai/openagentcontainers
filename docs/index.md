# Open Agent Containers

An **Open Agent Container** (OAC) is a standard Docker image annotated with
`org.openagentcontainers.v1.*` labels. The labels declare what the agent needs from its runtime
environment — the controlling infrastructure reads them and provisions resources accordingly.

## Concept

The labels form a negotiation between the artifact and the infrastructure:

- The **artifact author** (Dockerfile) declares requirements: what LLM it targets, what OAuth
  credentials it needs the orchestrator to provision, what filesystem mounts it needs, what
  environment variables it expects.
- The **infrastructure** (orchestrator) reads those declarations and decides how to satisfy them —
  which Ollama instance to point at, what volumes to mount, which env vars to inject.

The image itself contains everything needed to run the agent (the runtime harness, prompt,
extensions, tooling). The labels describe the agent's interface with the outside world.

## Label Namespace

All labels are namespaced under `org.openagentcontainers`. Configuration is declared under a major
version identifier like `v1`, which allows the spec to evolve without breaking existing tooling.

The current version is **`v0`** — this spec is still in beta.

## Example

```dockerfile
FROM node:25-alpine3.22

LABEL org.openagentcontainers.v1.name="pi-weather"
LABEL org.openagentcontainers.v1.inference.provider="ollama"
LABEL org.openagentcontainers.v1.inference.model="$OLLAMA_MODEL"

RUN npm install -g @mariozechner/pi-coding-agent
# ... rest of build
```

This agent has no workspace (it doesn't operate on files) and no external MCP servers (the weather
tool is implemented as a native extension inside the image).

## Inspecting Labels

```bash
# on a local image
docker image inspect pi-weather --format '{{json .Config.Labels}}' | jq

# on a remote image (no pull required)
docker manifest inspect ghcr.io/org/pi-weather:latest
```

Infrastructure reading labels at runtime should prefix-scan for `org.openagentcontainers.v1.` and
parse the structured fields from the flat key hierarchy.

## What the Spec Does Not Cover

The following are intentionally left to the runtime harness inside the image:

- **Prompt / system prompt** — an implementation detail of the harness, not observable from outside the image.
- **Skills and tools** — internal to the harness.
- **MCP server configuration** — how servers are launched or connected to is handled entirely by the harness. Labels only appear for MCPs that require OAuth credential negotiation.
- **Input/output channel schemas** — dropped for now; may be revisited.

See the [label reference](reference.md) for detailed documentation of every label.
