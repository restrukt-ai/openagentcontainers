# Label reference

All labels are namespaced under `org.openagentcontainers.v1`.

---

## Identity

```dockerfile
LABEL org.openagentcontainers.v1.name="my-agent"
```

The agent name. Used by the orchestrator to key agent-specific configuration (e.g. MCP credential
lookups).

---

## Inference

Declares what inference capabilities the agent requires. The orchestrator validates that the
configured inference gateway satisfies all declared requirements before deploying the agent.

**Connection** (required if any inference type is declared):

```dockerfile
LABEL org.openagentcontainers.v1.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.v1.inference.api_key.env="OPENAI_API_KEY"
```

| Label | Description |
|---|---|
| `inference.api_base.env` | Env var name the orchestrator MUST inject the gateway base URL into. |
| `inference.api_key.env` | Env var name the orchestrator MUST inject the API key into. |

The gateway must expose an OpenAI-compatible API (`POST /v1/chat/completions`, etc.).

**Per-type model requirements:**

```dockerfile
LABEL org.openagentcontainers.v1.inference.<type>.models="<model-id> [<model-id> ...]"
```

`<type>` is derived from the OpenAI API endpoint path: strip `/v1/` and replace `/` with `-`.

| Type key | OpenAI endpoint |
|---|---|
| `chat-completions` | `POST /v1/chat/completions` |
| `embeddings` | `POST /v1/embeddings` |
| `images-generations` | `POST /v1/images/generations` |
| `audio-speech` | `POST /v1/audio/speech` |
| `audio-transcriptions` | `POST /v1/audio/transcriptions` |
| `moderations` | `POST /v1/moderations` |

`models` is a space-separated list of model identifiers. **All listed models must be available**
on the configured gateway — the orchestrator validates each at deploy time and fails deployment if
any are missing. Undeclared types receive no validation and the harness must not use them.

**Example:**

```dockerfile
LABEL org.openagentcontainers.v1.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.v1.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.v1.inference.chat-completions.models="gpt-4o llama-3.1-8b-instruct"
LABEL org.openagentcontainers.v1.inference.embeddings.models="text-embedding-3-small"
```

---

## MCP Credentials

MCP server configuration — how to launch or connect to a server — is an implementation detail of
the runtime harness inside the image and is not declared in labels.

The only MCP concern the spec addresses is credential negotiation. An agent MAY declare one or more
auth methods per MCP. The auth method is expressed as a sub-namespace rather than a value, so an
agent can declare support for multiple methods simultaneously — the orchestrator satisfies whichever
it is configured to provide.

MCP servers that require no auth have no labels — they are entirely internal to the harness.

The orchestrator resolves the auth server URL and IAT from its own config, keyed by agent name and
MCP name — for an agent named `my-agent` with an MCP named `calendar`, it looks up
`my-agent/calendar`. The artifact does not encode infrastructure-specific URLs.

### DCR

The orchestrator performs Dynamic Client Registration ([RFC 7591](https://www.rfc-editor.org/rfc/rfc7591))
using an IAT it holds, then delivers the resulting credentials to the container.

| Label | Description |
|---|---|
| `mcp.<name>.dcr.scopes` | Space-separated OAuth scopes to request during registration, per [RFC 6749 §3.3](https://www.rfc-editor.org/rfc/rfc6749#section-3.3). |
| `mcp.<name>.dcr.client_id.env` | Env var name the orchestrator MUST inject the registered client ID into. |
| `mcp.<name>.dcr.client_id.file` | Path the orchestrator MUST write the registered client ID to. |
| `mcp.<name>.dcr.client_secret.env` | Env var name the orchestrator MUST inject the registered client secret into. |
| `mcp.<name>.dcr.client_secret.file` | Path the orchestrator MUST write the registered client secret to. |

```dockerfile
LABEL org.openagentcontainers.v1.mcp.calendar.dcr.scopes="calendar:read calendar:write"
LABEL org.openagentcontainers.v1.mcp.calendar.dcr.client_id.env="CALENDAR_CLIENT_ID"
LABEL org.openagentcontainers.v1.mcp.calendar.dcr.client_id.file="/run/secrets/calendar_client_id"
LABEL org.openagentcontainers.v1.mcp.calendar.dcr.client_secret.env="CALENDAR_CLIENT_SECRET"
LABEL org.openagentcontainers.v1.mcp.calendar.dcr.client_secret.file="/run/secrets/calendar_client_secret"
```

### OAuth

The client is pre-registered. The orchestrator injects a pre-configured `client_id` and
`client_secret` with no registration step.

| Label | Description |
|---|---|
| `mcp.<name>.oauth.client_id.env` / `.file` | Where the orchestrator delivers the client ID. |
| `mcp.<name>.oauth.client_secret.env` / `.file` | Where the orchestrator delivers the client secret. |

```dockerfile
LABEL org.openagentcontainers.v1.mcp.calendar.oauth.client_id.env="CALENDAR_CLIENT_ID"
LABEL org.openagentcontainers.v1.mcp.calendar.oauth.client_secret.env="CALENDAR_CLIENT_SECRET"
```

### Bearer

The orchestrator injects a static token.

| Label | Description |
|---|---|
| `mcp.<name>.bearer.token.env` | Env var name the orchestrator MUST inject the token into. |
| `mcp.<name>.bearer.token.file` | Path the orchestrator MUST write the token to. |

```dockerfile
LABEL org.openagentcontainers.v1.mcp.calendar.bearer.token.env="CALENDAR_TOKEN"
LABEL org.openagentcontainers.v1.mcp.calendar.bearer.token.file="/run/secrets/calendar_token"
```

At least one of `.env` or `.file` MUST be declared for each credential. Both MAY be declared
simultaneously, in which case the orchestrator MUST satisfy both.

---

## Workspaces

Declares filesystem mounts the agent can operate on. The infrastructure MUST respect the `mutable`
constraint — a `mutable=true` workspace MUST be mounted read-write; a workspace where `mutable` is
absent or `false` MUST be mounted read-only.

An agent MAY declare multiple workspaces.

| Label | Description |
|---|---|
| `workspace.<name>.path` | Mount path inside the container. Required. |
| `workspace.<name>.mutable` | `"true"` for read-write, `"false"` or absent for read-only. |

```dockerfile
LABEL org.openagentcontainers.v1.workspace.project.path="/workspace"
LABEL org.openagentcontainers.v1.workspace.project.mutable="true"
```

`mutable` defaults to `false` when absent.

When running on Kubernetes, mutable workspaces can be satisfied efficiently using PVC cloning. On
storage backends that support thin clones (Ceph RBD, Longhorn, OpenEBS ZFS, NetApp ONTAP), the
clone uses copy-on-write semantics so data is not duplicated until written. For a reusable,
immutable template, prefer snapshotting the template PVC and creating workspace PVCs from the
snapshot.

---

## Orchestrator

Required. Declares how the harness connects to the orchestrator at startup. The orchestrator
injects its address via the declared env var; the harness uses it to establish a connection on
startup.

```dockerfile
LABEL org.openagentcontainers.v1.orchestrator.env="ORCHESTRATOR_ADDR"
```

The harness MUST declare at least one auth method for the orchestrator connection. Auth methods
follow the same sub-namespace pattern as MCP credentials — declare whichever methods the harness
supports; the orchestrator satisfies one.

### Bearer

The orchestrator injects a short-lived signed token. The harness sends it as
`Authorization: Bearer <token>` metadata on the stream.

| Label | Description |
|---|---|
| `orchestrator.bearer.token.env` | Env var name the orchestrator MUST inject the token into. |
| `orchestrator.bearer.token.file` | Path the orchestrator MUST write the token to. |

```dockerfile
LABEL org.openagentcontainers.v1.orchestrator.bearer.token.env="ORCHESTRATOR_TOKEN"
LABEL org.openagentcontainers.v1.orchestrator.bearer.token.file="/run/secrets/orchestrator_token"
```

At least one of `.env` or `.file` MUST be declared. Both MAY be declared simultaneously, in which
case the orchestrator MUST satisfy both.

### mTLS

The orchestrator provisions a client certificate signed by its CA. Both sides present certificates
during the TLS handshake — the cert is the identity, no separate token is needed.

| Label | Description |
|---|---|
| `orchestrator.mtls.cert.file` | Path the orchestrator MUST write the client certificate (PEM) to. |
| `orchestrator.mtls.key.file` | Path the orchestrator MUST write the client private key (PEM) to. |
| `orchestrator.mtls.ca.file` | Path the orchestrator MUST write its CA certificate (PEM) to. Used by the harness to verify the orchestrator's server cert. |

```dockerfile
LABEL org.openagentcontainers.v1.orchestrator.mtls.cert.file="/run/secrets/harness.crt"
LABEL org.openagentcontainers.v1.orchestrator.mtls.key.file="/run/secrets/harness.key"
LABEL org.openagentcontainers.v1.orchestrator.mtls.ca.file="/run/secrets/ca.crt"
```

mTLS here is managed entirely by the orchestrator — it acts as the CA, signs a client certificate
for the harness instance at startup, and its gRPC server validates that certificate at connection
time. No service mesh is required.

A service mesh (Istio, Linkerd) MAY be used alongside or instead of label-based mTLS. When a mesh
provides transparent mTLS at the network layer, neither the harness nor the orchestrator application
code handles certificates directly, and no auth labels are needed. The two approaches are
independent: the mesh is a deployment concern, not a spec concern.

---

## Event Subscriptions

Declares the application-level event channels the agent subscribes to. Each channel is declared
with a schema file embedded in the image — the orchestrator extracts and caches these schemas at
registration time so it can configure event transformation before the agent starts.

Channel names must conform to the DNS label standard (RFC 1123): lowercase alphanumeric characters
and `-`, starting with an alphabetic character, ending with an alphanumeric character, max 63
characters.

Each channel declaration requires two labels:

| Label | Description |
|---|---|
| `events.<name>.schema.path` | Path to the schema file within the image. |
| `events.<name>.schema.mimetype` | MIME type of the schema file (e.g. `application/schema+json`, `application/protobuf`). |

Both labels are required per channel. The schema file must be present in the image at the declared
path at build time. Schema files may be placed at any path within the image.

```dockerfile
LABEL org.openagentcontainers.v1.events.pagerduty-alert.schema.path="/oaa/schemas/pagerduty-alert.json"
LABEL org.openagentcontainers.v1.events.pagerduty-alert.schema.mimetype="application/schema+json"

LABEL org.openagentcontainers.v1.events.workflow-response.schema.path="/oaa/schemas/workflow-response.pb"
LABEL org.openagentcontainers.v1.events.workflow-response.schema.mimetype="application/protobuf"
```

### Registration

The orchestrator inspects the image once at registration time:

1. Reads labels to collect `{ channel_name → { path, mimetype } }` mappings.
2. Extracts each declared schema file from the image.
3. Caches the schemas keyed by channel name.

The orchestrator uses these cached schemas to configure event transformation — mapping incoming
receiver payloads to the format the harness expects — before the agent starts.
