# Open Agent Containers

An **Open Agent Container** (OAC) is a standard Docker image annotated with
`org.openagentcontainers.*` labels. The labels declare what the agent needs from its runtime
environment — the controlling infrastructure reads them and provisions resources accordingly.

## Concept

The labels form a negotiation between the artifact and the infrastructure:

- The **artifact author** (Dockerfile) declares requirements: what inference capabilities it needs,
  what OAuth credentials it needs the orchestrator to provision, what filesystem mounts it needs,
  what environment variables it expects.
- The **infrastructure** (orchestrator) reads those declarations and decides how to satisfy them —
  which inference gateway to point at, what volumes to mount, which env vars to inject.

The image itself contains everything needed to run the agent (the runtime harness, prompt,
extensions, tooling). The labels describe the agent's interface with the outside world.

## Label Namespace

All labels are namespaced under `org.openagentcontainers`.

A docker image can declare itself to be an OAC compliant container by declaring its agent name and
spec version. See the [label reference](reference.md) for all available labels.

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

## What the Spec Does Not Cover

The following are left to the runtime harness inside the image:

- **Prompt / system prompt**
- **Skills and tools**
- **MCP server configuration** — labels only cover credential negotiation, not how servers are launched or connected.
- **Event payload format** — the spec declares schemas for orchestrator-side transformation; wire format is an implementation concern.
