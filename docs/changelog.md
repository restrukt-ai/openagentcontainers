# Changelog

All notable changes to the OAC specification are documented here.

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
