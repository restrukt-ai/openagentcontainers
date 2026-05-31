# Example: per-session medical prior authorization agent
#
# Reviews prior authorization requests against clinical guidelines and payer
# coverage policies, then produces a structured approval or denial decision.
# One container per session — PHI requires strict session isolation at the
# infrastructure level, not just in-process demultiplexing.
#
# Demonstrates:
#   - per-session container lifecycle for PHI workloads
#   - read-only workspace (clinical guidelines reference)
#   - mutable workspace (auth decision output)
#   - MCP credentials: DCR with FHIR scopes (EHR), bearer (payer rules engine)
#   - inference: chat completions + embeddings (guideline retrieval)
#   - event subscription: auth-request

FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /agent .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates

# --- Identity ---
LABEL org.openagentcontainers.version="v1alpha3"
LABEL org.openagentcontainers.name="prior-auth-agent"

# --- Orchestrator connection (mTLS) ---
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_ADDR"
LABEL org.openagentcontainers.orchestrator.mtls.cert.file="/run/secrets/harness.crt"
LABEL org.openagentcontainers.orchestrator.mtls.key.file="/run/secrets/harness.key"
LABEL org.openagentcontainers.orchestrator.mtls.ca.file="/run/secrets/ca.crt"

# --- Inference ---
LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.tools="true"
LABEL org.openagentcontainers.inference.chat-completions.context="1000000"
LABEL org.openagentcontainers.inference.embeddings.context="2048"

# --- MCP: EHR system (DCR, FHIR-scoped) ---
LABEL org.openagentcontainers.mcp.ehr-system.dcr.scopes="patient:read encounter:read condition:read procedure:read medication:read"
LABEL org.openagentcontainers.mcp.ehr-system.dcr.client_id.env="EHR_CLIENT_ID"
LABEL org.openagentcontainers.mcp.ehr-system.dcr.client_secret.env="EHR_CLIENT_SECRET"

# --- MCP: Payer rules engine (bearer) ---
LABEL org.openagentcontainers.mcp.payer-rules.bearer.token.env="PAYER_RULES_TOKEN"

# --- Workspaces ---
# Clinical guidelines corpus mounted read-only; decisions written to output mount.
LABEL org.openagentcontainers.workspace.guidelines.path="/workspace/guidelines"
LABEL org.openagentcontainers.workspace.decisions.path="/workspace/decisions"
LABEL org.openagentcontainers.workspace.decisions.mutable="true"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.auth-request.schema.path="/oac/schemas/auth-request.json"
LABEL org.openagentcontainers.events.auth-request.schema.mimetype="application/schema+json"

COPY schemas/auth-request.json /oac/schemas/auth-request.json
COPY --from=builder /agent /app/agent

CMD ["/app/agent"]
