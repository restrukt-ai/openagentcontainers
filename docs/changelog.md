# Changelog

All notable changes to the OAC specification are documented here.

---

## v0.2.0 — 2026-05-12

### Changed

- **Label version updated from `v1` to `v1alpha1`.** The `org.openagentcontainers.version` label
  value now uses Kubernetes-style maturity stages to communicate spec stability. The progression
  is `v1alpha1` → … → `v1beta1` → … → `v1` (stable/GA), with a new major version resuming at
  `v2alpha1`.

- **Versioning scheme documented in §8.1.** Added a maturity stage table (Alpha / Beta / Stable)
  and explicit rules: orchestrators MUST NOT automatically accept a later alpha or beta revision —
  each accepted version must be explicitly declared.

---

## v0.1.0 — 2026-05-04

Initial draft. Backported from the `docs/` directory.

### Defined

- OCI label namespace (`org.openagentcontainers.*`) and the explicit `version` label
- Label groups: `name`, `inference`, `mcp`, `workspace`, `orchestrator`, `events`
- Conformance classes: Producer, Orchestrator, Harness
- Error handling requirements (§7): missing labels, model validation failure, missing schema
  files, unsatisfiable auth methods, unknown labels, unsupported spec version
- Versioning and deprecation policy (§8): SemVer for the spec document, major version label for
  artifact negotiation, deprecation window of one minor version + six months
- Security considerations: artifact integrity, dependency trust, credential injection, agent
  identity
- Operational considerations: observability, registry compatibility, Kubernetes deployment,
  registration-time performance
- Appendix A examples: minimal artifact and full-featured artifact with MCP, workspace, and
  event subscription
- Appendix B implementation notes: schema file extraction via `crane`, label parsing guidance
