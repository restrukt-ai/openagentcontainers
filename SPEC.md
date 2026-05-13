---

# Open Agent Containers Specification

**Status:** Draft

**Version:** v1alpha2

**Authors:** [Brahm Lower](https://github.com/brahmlower) ([brahm@restrukt.ai](mailto:brahm@restrukt.ai))

**Created:** 2026-04-03

**Last Updated:** 2026-05-12

---

## Abstract

This document specifies the Open Agent Containers (OAC) format: a set of OCI image label
conventions and embedded file requirements that allow an AI agent artifact to declare its runtime
dependencies to an orchestrator. A conformant artifact communicates — via Docker labels baked into
the image at build time — its inference gateway requirements, MCP credential needs, filesystem
workspace requirements, orchestrator connection parameters, and event channel subscriptions. A
conformant orchestrator reads these declarations and satisfies them at deployment time without
requiring per-agent configuration outside the image. This specification is orchestrator-agnostic
and harness-agnostic.

---

## Status of This Document

This document is a **DRAFT** specification. It is the author's intent to propose this work to
the CNCF for formal adoption; formal engagement with the CNCF TOC has not yet begun. This
document does not represent the position of the CNCF TOC or any TAG.

Feedback is solicited via GitHub issues against
[https://github.com/restrukt-ai/openagentcontainers](https://github.com/restrukt-ai/openagentcontainers).
Implementation experience against draft versions is welcomed and will inform the document before
a CNCF proposal is made.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Terminology](#2-terminology)
3. [Relationship to Existing Work](#3-relationship-to-existing-work)
4. [Agent Artifact Format](#4-agent-artifact-format)
5. [Label Reference](#5-label-reference)
6. [Conformance](#6-conformance)
7. [Error Handling](#7-error-handling)
8. [Versioning and Compatibility](#8-versioning-and-compatibility)
9. [Security Considerations](#9-security-considerations)
10. [Operational Considerations](#10-operational-considerations)
11. [Alternatives Considered](#11-alternatives-considered)
12. [References](#12-references)

- [Appendix A. Examples](#appendix-a-examples)
- [Appendix B. Implementation Notes](#appendix-b-implementation-notes)
- [Acknowledgements](#acknowledgements)
- [Revision History](#revision-history)

---

## 1. Introduction

### 1.1 Motivation

An AI agent is more than a model and a prompt. At runtime it needs a live inference endpoint,
credentials for the external services it uses, filesystem storage to operate on, and a channel
back to the orchestrator that launched it. These are infrastructure concerns — and how they are
satisfied changes between environments (local dev, staging, production, multi-tenant SaaS).

The naive solution is to bake this configuration into the image itself or pass it through ad-hoc
environment variables documented in a README. Both approaches break portability: the image becomes
coupled to a specific deployment, and the orchestrator has no structured way to know what the
agent actually needs before it runs.

OAC inverts this model. The agent image declares its own requirements as OCI image labels. A
compliant orchestrator in any environment reads those labels and satisfies them — without out-of-band
configuration files or per-agent knowledge hardcoded into the infrastructure.

### 1.2 Goals

- Define a machine-readable, orchestrator-agnostic contract for AI agent runtime dependencies
  expressed entirely as OCI image labels.
- Enable any compliant orchestrator to deploy any compliant agent image without per-agent
  configuration outside the image.
- Allow agent images to be stored in any OCI-compatible registry and scheduled by any orchestrator
  that reads their labels.
- Make agents first-class infrastructure primitives within existing cloud-native ecosystems —
  participating in existing registry workflows, container orchestration, and observability pipelines.
- Remain harness-agnostic: impose no constraints on the language, framework, or SDK used to
  implement the agent's reasoning loop.

### 1.3 Non-Goals

- This specification does not define agent composition (language, SDKs, MCP tooling, or skills).
- This specification does not define agent runtime behavior or execution semantics.
- This specification does not define a new container runtime or OCI distribution mechanism.
- This specification does not prescribe inter-agent communication protocols (MCP, A2A, etc.).
- This specification does not define model weight packaging; see ModelPack
  ([§11.1](#111-modelpack--modelkit)) for that scope.
- This specification does not restrict the inference platform used by the orchestrator.

---

## 2. Terminology

### 2.1 Key Words

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT",
"RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted
as described in BCP 14 [[RFC2119]] [[RFC8174]] when, and only when, they appear in all capitals,
as shown here.

### 2.2 Definitions

| Term                        | Definition                                                                                                                              |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| **Agent Artifact**          | An OCI image conforming to this specification that packages an AI agent and declares its runtime dependencies via labels                |
| **Producer**                | An entity that creates and publishes a conformant Agent Artifact (typically a Dockerfile author or CI pipeline)                         |
| **Consumer / Orchestrator** | An entity that ingests an Agent Artifact, reads its labels, and provisions the declared dependencies at deploy time                     |
| **Harness**                 | The process inside the container that drives the agent's reasoning loop; must satisfy the runtime interface requirements in §4.3        |
| **Label Namespace**         | The `org.openagentcontainers` prefix under which all OAC labels are declared                                                            |
| **Dependency Declaration**  | One or more labels in the Label Namespace that describe a resource the agent requires at runtime                                        |
| **Event Channel**           | A named application-level event stream the agent subscribes to, declared with a schema file embedded in the image                       |
| **Schema File**             | A file embedded in the image at build time that describes the payload format for an event channel                                       |
| **Registration**            | The one-time process by which an orchestrator inspects an image's labels and extracts cached schema files, prior to any container start |
| **OCI Image**               | A container image conforming to the [OCI Image Format Specification]                                                                    |

---

## 3. Relationship to Existing Work

_This section is informative._

### 3.1 OCI Image Specification

This specification builds on the [OCI Image Format Specification] and [OCI Distribution
Specification]. Agent Artifacts are valid OCI images; this specification defines additional
constraints on label content and required embedded files on top of the base OCI image format.

### 3.2 ModelPack / ModelKit (TOC Initiative #1740)

[TOC initiative #1740] targets convergence on a single OCI manifest format for AI model weights,
metadata, and SBOMs. This specification addresses a distinct artifact type — a runnable agent,
not a model weight bundle — but the two are composable: an Agent Artifact MAY reference a
ModelPack artifact as an inference dependency.

### 3.3 Agent CRD and Runtime Abstraction (TOC Initiative #1746)

[TOC initiative #1746] is focused on an Agent CRD schema and lifecycle model for Kubernetes.
OAC labels are intended to serve as the artifact-level source of truth that an orchestrator reads
when instantiating such a CRD: OAC defines what the artifact declares; a CRD-based system defines
how Kubernetes represents and manages the running agent.

### 3.4 Cloud-Native Agentic Standards Checklist (TOC Initiative #1749)

[TOC initiative #1749] is defining container and observability best practices for agentic systems.
The Security Considerations and Operational Considerations sections of this specification are
informed by that scope.

---

## 4. Agent Artifact Format

### 4.1 OCI Manifest Requirements

An Agent Artifact is a valid OCI image. All OAC-specific metadata is expressed as OCI image
labels — key-value string pairs embedded in the image at build time. No additional manifest
types or custom media types are required at the OCI level.

A conformant Agent Artifact MUST include the following label:

```dockerfile
LABEL org.openagentcontainers.version="v1alpha2"
```

This label declares the spec version the artifact conforms to. The orchestrator reads this
label first to determine whether it can process the artifact. The value MUST be a single version
identifier (e.g., `"v1alpha2"`).

All OAC labels are namespaced under `org.openagentcontainers`. Labels outside this namespace are
not governed by this specification and MUST be ignored by conformant orchestrators when performing
OAC-specific processing.

### 4.2 Label Conventions

Label keys follow a hierarchical dot-separated structure:

```
org.openagentcontainers.<group>[.<name>].<attribute>[.<sub-attribute>]
```

Where `<group>` is one of: `name`, `inference`, `mcp`, `workspace`, `orchestrator`, `events`.

Label values are UTF-8 strings. Where a label accepts multiple values, they are
space-separated within a single string. Where a label references an environment variable
name or file path, the value is the name or path string — not the resolved value. Resolving
and injecting values at deploy time is the orchestrator's responsibility.

### 4.3 Embedded Files

Agent Artifacts MAY embed files in their image layers. When event subscriptions are declared
(§5.5), the corresponding schema files MUST be present in the image at the declared paths at
build time. Schema files MUST NOT be generated at container startup.

The orchestrator extracts these files from the image without running the container, using direct
OCI registry access against the image layers.

### 4.4 Harness Runtime Interface

OAC does not prescribe the internal implementation of the agent harness. However, a harness
MUST satisfy the following interface requirements to participate in the OAC ecosystem:

**Orchestrator connection** — The harness MUST initiate an outbound ConnectRPC bidirectional
stream to the address injected via the env var declared in `orchestrator.env`. If the image
declares an auth method (`orchestrator.bearer.*` or `orchestrator.mtls.*`), the harness MUST use
the injected credentials to authenticate the stream. Establishing the stream is the signal that
the harness is ready to receive events — no separate registration message is required. Compliant
harnesses MUST support at least one of the Connect, gRPC, or gRPC-Web protocols.

**Event schema files** — If the image declares event subscriptions (§5.5), each schema file MUST
be present in the image at its declared path at build time.

---

## 5. Label Reference

### 5.1 Identity

```dockerfile
LABEL org.openagentcontainers.name="my-agent"
```

The `name` label is REQUIRED. It identifies the agent and is used by the orchestrator to key
agent-specific configuration (e.g., MCP credential lookups).

| Label  | Required | Description                                                                          |
| ------ | -------- | ------------------------------------------------------------------------------------ |
| `name` | Yes      | Human-readable agent identifier. Used as a key for orchestrator-side config lookups. |

### 5.2 Inference

Declares what inference capabilities the agent requires. A conformant Orchestrator MUST only satisfy inference declarations using a gateway that exposes
an OpenAI-compatible API.

**Connection labels** — REQUIRED if any inference type is declared:

| Label                    | Required    | Description                                                          |
| ------------------------ | ----------- | -------------------------------------------------------------------- |
| `inference.api_base.env` | Conditional | Env var name the orchestrator MUST inject the gateway base URL into. |
| `inference.api_key.env`  | Conditional | Env var name the orchestrator MUST inject the API key into.          |

Both connection labels MUST be declared together. An image MUST NOT declare one without the other.

**Per-type model requirements:**

```
org.openagentcontainers.inference.<type>.models="<model-id> [<model-id> ...]"
```

`<type>` is derived from the OpenAI API endpoint path: strip `/v1/` and replace `/` with `-`.

| Type key               | OpenAI endpoint                 |
| ---------------------- | ------------------------------- |
| `chat-completions`     | `POST /v1/chat/completions`     |
| `embeddings`           | `POST /v1/embeddings`           |
| `images-generations`   | `POST /v1/images/generations`   |
| `audio-speech`         | `POST /v1/audio/speech`         |
| `audio-transcriptions` | `POST /v1/audio/transcriptions` |
| `moderations`          | `POST /v1/moderations`          |

The value is a space-separated list of model identifiers. The orchestrator MUST validate that all
listed models are available on the configured gateway before deployment, and MUST fail deployment
if any are missing. The harness MUST NOT use inference types that are not declared.

**Example:**

```dockerfile
LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.models="gpt-4o llama-3.1-8b-instruct"
LABEL org.openagentcontainers.inference.embeddings.models="text-embedding-3-small"
```

### 5.3 MCP Credentials

MCP server configuration — how to launch or connect to a server — is an implementation detail of
the harness and is not declared in labels. The only MCP concern this specification addresses is
credential negotiation.

An agent MAY declare one or more auth methods per MCP server. Auth methods are expressed as
sub-namespaces so an agent can declare support for multiple methods simultaneously; the
orchestrator satisfies whichever it is configured to provide. MCP servers that require no auth
have no labels.

The orchestrator resolves auth server URLs and initial access tokens from its own configuration,
keyed by `<agent-name>/<mcp-name>`. The artifact does not encode infrastructure-specific URLs.

**DCR (Dynamic Client Registration, [RFC 7591]):**

| Label                               | Description                                                                  |
| ----------------------------------- | ---------------------------------------------------------------------------- |
| `mcp.<name>.dcr.scopes`             | Space-separated OAuth scopes to request, per [RFC 6749 §3.3].                |
| `mcp.<name>.dcr.client_id.env`      | Env var name the orchestrator MUST inject the registered client ID into.     |
| `mcp.<name>.dcr.client_id.file`     | Path the orchestrator MUST write the registered client ID to.                |
| `mcp.<name>.dcr.client_secret.env`  | Env var name the orchestrator MUST inject the registered client secret into. |
| `mcp.<name>.dcr.client_secret.file` | Path the orchestrator MUST write the registered client secret to.            |

**OAuth (pre-registered client):**

| Label                                          | Description                                        |
| ---------------------------------------------- | -------------------------------------------------- |
| `mcp.<name>.oauth.client_id.env` / `.file`     | Where the orchestrator delivers the client ID.     |
| `mcp.<name>.oauth.client_secret.env` / `.file` | Where the orchestrator delivers the client secret. |

**Bearer token:**

| Label                          | Description                                               |
| ------------------------------ | --------------------------------------------------------- |
| `mcp.<name>.bearer.token.env`  | Env var name the orchestrator MUST inject the token into. |
| `mcp.<name>.bearer.token.file` | Path the orchestrator MUST write the token to.            |

For each credential, at least one of `.env` or `.file` MUST be declared. Both MAY be declared
simultaneously, in which case the orchestrator MUST satisfy both.

### 5.4 Workspaces

Declares filesystem mounts the agent operates on.

| Label                      | Required | Description                                                 |
| -------------------------- | -------- | ----------------------------------------------------------- |
| `workspace.<name>.path`    | Yes      | Mount path inside the container.                            |
| `workspace.<name>.mutable` | No       | `"true"` for read-write. Absent or `"false"` for read-only. |

The orchestrator MUST respect the `mutable` constraint: a `mutable=true` workspace MUST be
mounted read-write; a workspace where `mutable` is absent or `"false"` MUST be mounted read-only.
An agent MAY declare multiple workspaces.

### 5.5 Orchestrator Connection

The `orchestrator` group is REQUIRED. It declares how the harness connects to the orchestrator.

| Label              | Required | Description                                                 |
| ------------------ | -------- | ----------------------------------------------------------- |
| `orchestrator.env` | Yes      | Env var name the orchestrator MUST inject its address into. |

The harness MUST declare at least one auth method. Auth methods follow the same sub-namespace
pattern as MCP credentials; the orchestrator satisfies one.

**Bearer:**

| Label                            | Description                                               |
| -------------------------------- | --------------------------------------------------------- |
| `orchestrator.bearer.token.env`  | Env var name the orchestrator MUST inject the token into. |
| `orchestrator.bearer.token.file` | Path the orchestrator MUST write the token to.            |

The harness sends the token as `Authorization: Bearer <token>` on the ConnectRPC stream.

**mTLS:**

| Label                         | Description                                                                  |
| ----------------------------- | ---------------------------------------------------------------------------- |
| `orchestrator.mtls.cert.file` | Path the orchestrator MUST write the client certificate (PEM) to.            |
| `orchestrator.mtls.key.file`  | Path the orchestrator MUST write the client private key (PEM) to.            |
| `orchestrator.mtls.ca.file`   | Path the orchestrator MUST write the orchestrator's CA certificate (PEM) to. |

The orchestrator acts as the CA, signs a client certificate for the harness at startup, and
validates that certificate at connection time. No service mesh is required. A service mesh MAY
be used alongside label-based mTLS; the two are independent deployment concerns.

### 5.6 Event Subscriptions

Declares application-level event channels the agent subscribes to. Each channel is declared with
a schema file embedded in the image; the orchestrator extracts and caches these files at
registration time.

Channel names MUST conform to the DNS label format ([RFC 1123]): lowercase alphanumeric
characters and hyphens, starting with an alphabetic character, ending with an alphanumeric
character, maximum 63 characters.

| Label                           | Required          | Description                               |
| ------------------------------- | ----------------- | ----------------------------------------- |
| `events.<name>.schema.path`     | Yes (per channel) | Path to the schema file within the image. |
| `events.<name>.schema.mimetype` | Yes (per channel) | MIME type of the schema file.             |

Both labels are REQUIRED per channel declaration. The schema file MUST be present in the image
at the declared path at build time.

Supported MIME types include `application/schema+json` and `application/protobuf`. Additional
types MAY be supported by specific orchestrator implementations.

**Registration flow:**

The orchestrator inspects the image once at registration time (not at every cold start):

1. Reads labels to collect `{ channel_name → { path, mimetype } }` mappings.
2. Extracts each declared schema file from the image without running it.
3. Caches the schemas keyed by channel name.

The orchestrator uses these cached schemas to configure event transformation before the agent
starts.

---

## 6. Conformance

Three conformance classes are defined: Container, Orchestrator, and Harness.

### 6.1 Container Conformance

A conformant Container MUST:

- Include the `org.openagentcontainers.version` label set to the spec version the artifact
  targets (e.g., `"v1alpha2"`).
- Include the `org.openagentcontainers.name` label.
- Include the `org.openagentcontainers.orchestrator.env` label.
- Declare at least one orchestrator auth method (`orchestrator.bearer.*` or `orchestrator.mtls.*`).
- If declaring any inference label, include both `inference.api_base.env` and `inference.api_key.env`.
- If declaring an event channel, include both `events.<name>.schema.path` and
  `events.<name>.schema.mimetype` for that channel.
- Ensure all declared schema files are present in the image at their declared paths at build time.
- Use valid DNS label names ([RFC 1123]) for event channel names.
- Not declare inference types the harness does not use.

### 6.2 Orchestrator Conformance

A conformant Orchestrator MUST:

- Read `org.openagentcontainers.version` before processing any other OAC labels.
- Fail deployment if `org.openagentcontainers.version` is absent or declares a version the
  orchestrator does not support, with a diagnostic identifying the unsupported version.
- Read all `org.openagentcontainers.*` labels from Agent Artifacts before deployment.
- Inject the inference gateway base URL and API key into the env vars declared by
  `inference.api_base.env` and `inference.api_key.env`.
- Validate that all models listed in `inference.<type>.models` are available on the configured
  gateway before deployment, and MUST fail deployment if any are unavailable.
- Perform Dynamic Client Registration ([RFC 7591]) using its IAT for each MCP server declaring
  `mcp.<name>.dcr.*`, and deliver the resulting credentials to the declared env vars or files.
- Deliver pre-registered OAuth credentials for each MCP server declaring `mcp.<name>.oauth.*`.
- Deliver bearer tokens for each MCP server declaring `mcp.<name>.bearer.*`.
- Mount workspaces at declared paths and respect the `mutable` constraint.
- Inject the orchestrator address into the env var declared by `orchestrator.env`.
- Satisfy at least one declared orchestrator auth method.
- Extract declared schema files from the image without running the container.
- Cache schema files keyed by channel name at registration time.

A conformant Orchestrator MAY:

- Use any OCI-compatible mechanism to extract schema files from image layers.
- Support MIME types beyond `application/schema+json` and `application/protobuf`.

### 6.3 Harness Conformance

A conformant Harness MUST:

- Initiate an outbound ConnectRPC bidirectional stream to the orchestrator address injected via
  the declared `orchestrator.env` env var at startup.
- Authenticate the stream using the injected credentials if an auth method is declared.
- Support at least one of the Connect, gRPC, or gRPC-Web protocols.
- Not use inference types that are not declared in the image labels.

Implementations MAY claim partial conformance only with respect to a named conformance class and
MUST NOT claim full conformance unless all normative requirements of that class are satisfied.

---

## 7. Error Handling

### 7.1 Missing Required Labels

A conformant Orchestrator MUST refuse to deploy an Agent Artifact that is missing required labels.
At minimum: if `org.openagentcontainers.name` or `org.openagentcontainers.orchestrator.env`
is absent, deployment MUST fail with a diagnostic indicating which label is missing.

### 7.2 Model Validation Failure

If a model listed in `inference.<type>.models` is not available on the configured gateway, the
orchestrator MUST fail deployment before the container starts. It MUST NOT start the container and
allow the harness to discover the missing model at runtime.

### 7.3 Missing Schema File

If an event channel is declared but its schema file is not present at the declared path in the
image, the orchestrator MUST fail registration and MUST NOT deploy the agent until the image is
corrected.

### 7.4 Unsatisfiable Auth Method

If an image declares orchestrator or MCP auth methods that the orchestrator cannot satisfy (e.g.,
declares only `mtls.*` but the orchestrator is not configured as a CA), the orchestrator MUST
fail deployment with a diagnostic indicating which auth method could not be satisfied.

### 7.5 Unknown Labels

A conformant Orchestrator MUST ignore labels under `org.openagentcontainers` that it does not
recognize. Unknown labels MUST NOT cause deployment failure.

### 7.6 Unsupported Spec Version

If `org.openagentcontainers.version` declares a version the orchestrator does not support, the
orchestrator MUST fail deployment. It MUST NOT attempt to process the artifact's dependency labels
or apply partial behavior based on labels it recognizes. The failure diagnostic MUST include the
version value declared in the artifact and the versions the orchestrator supports.

---

## 8. Versioning and Compatibility

### 8.1 Versioning via the Version Label

The spec version is declared via the `org.openagentcontainers.version` label (§4.1) and is the
authoritative mechanism for spec version negotiation between Producer and Orchestrator.

A breaking change to the label schema — removing a label, changing its semantics, or changing
required structure — requires a major version increment. The current spec version is `v1alpha2`.

The version value encodes both a major version number and a maturity stage:

| Stage       | Example values            | Meaning                                                                                                                     |
| ----------- | ------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Alpha       | `v1alpha1`, `v1alpha2`, … | Experimental. Label schema may change between revisions without notice.                                                     |
| Beta        | `v1beta1`, `v1beta2`, …   | Feature-complete for the major version. Breaking changes are avoided and announced in advance. Suitable for early adopters. |
| Stable (GA) | `v1`                      | Stable. Breaking changes require a new major version (`v2alpha1`).                                                          |

The graduation path for a major version is: `v1alpha1` → … → `v1beta1` → … → `v1`.
A new major version begins at `v2alpha1` and follows the same path.

Alpha and beta revisions within the same major version are distinct version values — an orchestrator
supporting `v1alpha1` MUST NOT automatically accept `v1alpha2` or `v1beta1`. Each accepted version
MUST be explicitly declared by the orchestrator.

### 8.2 Artifact Version Negotiation

A conformant orchestrator determines the spec version of an artifact by reading the explicit
`org.openagentcontainers.version` label (§4.1). This label is the sole source of truth for
version detection; the orchestrator MUST NOT infer the version from the label namespace alone.

An artifact MUST declare exactly one version. Multi-version coexistence — declaring labels from
multiple spec versions in a single image — is not permitted. When a new major version is released,
producers MUST publish a new image targeting the new version; they MUST NOT add new-version labels
alongside old-version labels in the same image.

If the declared version is not supported by the orchestrator, deployment MUST fail (§7.6). There
is no fallback or degraded-mode behavior for version mismatches.

### 8.3 Deprecation Policy

A label is deprecated when a preferred replacement is introduced. A conformant Orchestrator MUST
continue to support deprecated labels for at least **one spec revision AND six months** after the
revision that introduced the deprecation — whichever is longer. Deprecated labels are removed only
at a major version increment.

The spec document marks deprecated labels in the label reference tables with a "Deprecated in
`<version>`" annotation. The changelog for the deprecating revision will identify the deprecated
label and its replacement.

A conformant orchestrator SHOULD emit a warning at registration time when it encounters a label
documented as deprecated in the declared spec version. The warning MUST identify the deprecated
label key and, where a replacement exists, the replacement key. This warning MUST NOT prevent
deployment.

Build-time detection of deprecated labels is the responsibility of linting tooling, not the
orchestrator. The spec does not define a machine-readable deprecation signal in the image itself.

---

## 9. Security Considerations

### 9.1 Threat Model

<!--
  Define trust boundaries: Producer, registry, Orchestrator, Harness.
  Key threats: malicious artifact declaring excessive dependencies,
  credential injection interception, schema file tampering.
-->

### 9.2 Artifact Integrity and Provenance

Orchestrators SHOULD verify image signatures before reading labels or extracting schema files.
Producers SHOULD sign Agent Artifacts using Sigstore or Notary v2 and include an SBOM as an OCI
referrer.

### 9.3 Dependency Trust

The orchestrator provisions resources (credentials, volumes, network access) based solely on what
an artifact declares. A malicious or compromised artifact could declare dependencies that trigger
unwanted provisioning.

Orchestrators MUST enforce an allowlist or policy layer that restricts which inference gateways,
MCP servers, and workspace sources an artifact is permitted to declare. An artifact's declarations
are a request, not a grant.

### 9.4 Credential Injection

Credentials injected via environment variables or mounted files are accessible to all processes
in the container. Producers SHOULD prefer file-based delivery (`*.file` labels) over environment
variable delivery (`*.env` labels) for sensitive credentials, and SHOULD configure the harness to
clear credentials from memory after use.

### 9.5 Agent Identity

Each agent instance SHOULD be issued a unique workload identity (e.g., a SPIFFE SVID or
Kubernetes ServiceAccount token) scoped to its runtime lifetime. The orchestrator connection auth
(§5.5) is one mechanism for per-instance identity; orchestrators SHOULD issue short-lived,
automatically rotated credentials tied to container lifetime rather than static tokens.

---

## 10. Operational Considerations

### 10.1 Observability

Agent containers participate in existing observability pipelines. Orchestrators SHOULD emit
OpenTelemetry spans for the registration and startup lifecycle phases, keyed by agent name and
image digest. Harness implementors SHOULD follow the [OTel GenAI Agent Spans] semantic
conventions for agent-level traces.

### 10.2 Registry Compatibility

Agent Artifacts MUST be storable in any OCI-compatible registry (Harbor, ECR, GHCR, Docker Hub)
without modification. Labels and embedded schema files are part of the standard OCI image format
and require no registry extensions.

### 10.3 Kubernetes Deployment

The natural Kubernetes deployment pattern for a conformant orchestrator is a custom operator with
CRDs that represent desired agent state and reconcile it like any other workload. Mutable
workspaces (§5.4) can be efficiently satisfied using PVC cloning on storage backends that support
thin clones (Ceph RBD, Longhorn, OpenEBS ZFS, NetApp ONTAP).

### 10.4 Performance

Schema file extraction (§5.6 Registration) is a one-time cost at registration time, not at every
cold start. Orchestrators SHOULD cache extracted schemas by image digest to avoid redundant
registry fetches across deployments of the same image.

---

## 11. Alternatives Considered

### 11.1 ModelPack / ModelKit

_Why not just extend ModelPack for agents?_

ModelPack (TOC initiative #1740) targets model weight bundles — static artifacts that are not
directly runnable and do not have orchestrator connection requirements. An agent artifact is a
runnable container with a live connection back to its orchestrator. The dependency and lifecycle
models are sufficiently different that extending ModelPack would conflate two distinct artifact
types. The two formats are composable: an agent artifact may declare a ModelPack artifact as an
inference dependency.

### 11.2 OCI Image Annotations Only (No Embedded Files)

_Why not encode all metadata as OCI annotations without requiring embedded schema files?_

OCI annotation values are strings with a practical size limit (the OCI spec does not mandate a
maximum, but registries and runtimes impose one in practice). Event schemas — particularly
protobuf FileDescriptorSets with transitive imports — exceed this limit. Embedding files in image
layers is the standard OCI mechanism for arbitrary payloads and survives push, pull, and
re-tagging without modification.

### 11.3 Kubernetes Agent CRD as the Primary Interface

_Why not put dependency declarations in the CRD rather than the artifact?_

Putting declarations in a CRD decouples the dependency specification from the artifact. This
means the same image can be deployed with different declarations — undermining the portability
goal. It also requires out-of-band CRD authorship for every agent deployment. Labels in the
image make the artifact self-describing: any compliant orchestrator can deploy it without
additional configuration. A CRD-based system is the right place for orchestrator-side state; the image labels are the right
place for artifact-side declarations.

### 11.4 Structured Config File Inside the Image

_Why labels rather than a JSON/YAML config file baked into the image?_

A structured file inside the image requires either running the container or extracting and
parsing a file from image layers to discover requirements. OCI image labels are part of the image
manifest and are available from any registry API call that returns image metadata — no layer
extraction required for the common case. This makes registration-time inspection cheap and
enables orchestrators to read requirements without ever starting a container.

---

## 12. References

### 12.1 Normative References

| Label                            | Reference                                                                                                               |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| [RFC2119]                        | Bradner, S. "Key words for use in RFCs to Indicate Requirement Levels." BCP 14, RFC 2119. March 1997.                   |
| [RFC8174]                        | Leiba, B. "Ambiguity of Uppercase vs Lowercase in RFC 2119 Key Words." BCP 14, RFC 8174. May 2017.                      |
| [RFC1123]                        | Braden, R. "Requirements for Internet Hosts — Application and Support." RFC 1123. October 1989. (§2.1 DNS label format) |
| [RFC6749]                        | Hardt, D. "The OAuth 2.0 Authorization Framework." RFC 6749. October 2012.                                              |
| [RFC7591]                        | Richer, J. et al. "OAuth 2.0 Dynamic Client Registration Protocol." RFC 7591. July 2015.                                |
| [OCI Image Format Specification] | Open Container Initiative. "OCI Image Format Specification." https://github.com/opencontainers/image-spec               |
| [OCI Distribution Specification] | Open Container Initiative. "OCI Distribution Specification." https://github.com/opencontainers/distribution-spec        |

### 12.2 Informative References

| Label                    | Reference                                                                                                                                                           |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ConnectRPC]             | Buf Technologies. "Connect Protocol." https://connectrpc.com                                                                                                        |
| [SemVer]                 | Preston-Werner, T. "Semantic Versioning 2.0.0." https://semver.org                                                                                                  |
| [TOC #1740]              | Caldeira, V. et al. "Cloud Native and OCI Compliant Inner-Loop Tooling & Packaging for AI Engineers." CNCF TOC Issue #1740. https://github.com/cncf/toc/issues/1740 |
| [TOC #1746]              | Caldeira, V. et al. "Cloud-Native Foundations for Distributed Agentic Systems." CNCF TOC Issue #1746. https://github.com/cncf/toc/issues/1746                       |
| [TOC #1749]              | Halley, J. et al. "Cloud-Native Agentic Standards Checklist." CNCF TOC Issue #1749. https://github.com/cncf/toc/issues/1749                                         |
| [OTel GenAI Agent Spans] | OpenTelemetry. "Semantic Conventions for GenAI Agent Spans." https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/                                 |
| [SPIFFE]                 | SPIFFE Project. "Secure Production Identity Framework for Everyone." https://spiffe.io                                                                              |

---

## Appendix A. Examples

_Informative._

### A.1 Minimal Conformant Agent Artifact

```dockerfile
FROM node:22-alpine

LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="minimal-agent"

LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_ADDR"
LABEL org.openagentcontainers.orchestrator.bearer.token.env="ORCHESTRATOR_TOKEN"

LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.models="llama3.2"

COPY harness.js /app/harness.js
CMD ["node", "/app/harness.js"]
```

### A.2 Agent with MCP Credentials, Workspace, and Event Subscription

```dockerfile
FROM python:3.12-slim

LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="pi-weather"

LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_ADDR"
LABEL org.openagentcontainers.orchestrator.mtls.cert.file="/run/secrets/harness.crt"
LABEL org.openagentcontainers.orchestrator.mtls.key.file="/run/secrets/harness.key"
LABEL org.openagentcontainers.orchestrator.mtls.ca.file="/run/secrets/ca.crt"

LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.models="gpt-4o"
LABEL org.openagentcontainers.inference.embeddings.models="text-embedding-3-small"

LABEL org.openagentcontainers.mcp.calendar.dcr.scopes="calendar:read calendar:write"
LABEL org.openagentcontainers.mcp.calendar.dcr.client_id.env="CALENDAR_CLIENT_ID"
LABEL org.openagentcontainers.mcp.calendar.dcr.client_secret.env="CALENDAR_CLIENT_SECRET"

LABEL org.openagentcontainers.workspace.project.path="/workspace"
LABEL org.openagentcontainers.workspace.project.mutable="true"

LABEL org.openagentcontainers.events.pagerduty-alert.schema.path="/oaa/schemas/pagerduty-alert.json"
LABEL org.openagentcontainers.events.pagerduty-alert.schema.mimetype="application/schema+json"

COPY schemas/pagerduty-alert.json /oaa/schemas/pagerduty-alert.json
COPY agent.py /app/agent.py
CMD ["python", "/app/agent.py"]
```

---

## Appendix B. Implementation Notes

_Informative. Guidance for implementors beyond normative requirements._

### B.1 Extracting Schema Files from an Image

Compliant orchestrators extract schema files via direct OCI registry access against image layers.
The `crane` library (`github.com/google/go-containerregistry/pkg/crane`) provides a
straightforward implementation:

```bash
crane export ghcr.io/org/my-agent:latest - | tar xf - --to-stdout oaa/schemas/pagerduty-alert.json
```

### B.2 Label Parsing

Orchestrators parsing OAC labels SHOULD treat the label key hierarchy as a structured tree rather
than a flat map. Grouping labels by their second-level key (`inference`, `mcp`, `workspace`,
`orchestrator`, `events`) before processing each group reduces the risk of misattributing a label
to the wrong group.

---

## Acknowledgements

---

## Revision History

| Version  | Date       | Summary                                                                                                                           |
| -------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------- |
| v1alpha2 | 2026-05-12 | Adopt Kubernetes-style maturity stages; unify spec document version with label version; document graduation path and §8 overhaul. |
| v1alpha1 | 2026-05-04 | Initial draft; backported from docs/.                                                                                             |
