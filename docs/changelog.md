# Changelog

All notable changes to the OAC specification are documented here.

---

## v1alpha2 — 2026-05-20

### Added

- **Session isolation** (`session.isolation` label, §5.7). Declares whether the harness handles
  multiple concurrent sessions in a single process. When `"true"`, the orchestrator deploys the
  container as a long-running service; when absent or `"false"`, one container instance is started
  per session.

- **Stream message schema** (§4.5). A normative protobuf schema for the ConnectRPC stream between
  harness and orchestrator, with `HarnessEnvelope`, `OrchestratorEnvelope`, `Event`,
  `SessionEnd`, and `EventResult` messages.

- **`session_id` field on stream messages.** Every message on the stream MUST carry a non-empty
  `session_id`. Harnessess declaring `session.isolation="true"` MUST demultiplex concurrent
  sessions using this field.

- **Workspace/session conflict error condition** (§7.5). Orchestrators MUST fail deployment if
  `session.isolation="true"` and any `workspace.*` labels are declared together.

- **`session` label group** added to the label group enumeration (§4.2).

---

## v1alpha2 — 2026-05-12

### Changed

- **Spec document version now aligned with label version.** The document version and the
  `org.openagentcontainers.version` label value are the same identifier — there is no separate
  document versioning scheme (§8.2).

- **Label version updated from `v1` to `v1alpha2`.** The `org.openagentcontainers.version` label
  value now uses Kubernetes-style maturity stages to communicate spec stability. The progression
  is `v1alpha1` → … → `v1beta1` → … → `v1` (stable/GA), with a new major version resuming at
  `v2alpha1` (§8.1).

- **Orchestrator version acceptance rules.** Orchestrators MUST NOT automatically accept a later
  alpha or beta revision — each accepted version must be explicitly declared.

---

## v1alpha1 — 2026-05-04

Initial release.

### Defined

- OCI label namespace (`org.openagentcontainers.*`) and the explicit `version` label
- Label groups: `name`, `inference`, `mcp`, `workspace`, `orchestrator`, `events`
- Conformance classes: Producer, Orchestrator, Harness
- Error handling requirements (§7): missing labels, model validation failure, missing schema
  files, unsatisfiable auth methods, unknown labels, unsupported spec version
- Versioning and deprecation policy (§8)
- Security considerations: artifact integrity, dependency trust, credential injection, agent
  identity
- Operational considerations: observability, registry compatibility, Kubernetes deployment,
  registration-time performance
- Appendix A examples: minimal artifact and full-featured artifact with MCP, workspace, and
  event subscription
- Appendix B implementation notes: schema file extraction via `crane`, label parsing guidance
