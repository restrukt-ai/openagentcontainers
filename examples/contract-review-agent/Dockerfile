# Example: per-session contract review agent
#
# Reviews inbound contracts against company standard templates, flags
# non-standard clauses and risk factors, and produces a tracked-changes
# redline document ready for legal sign-off.
#
# Demonstrates:
#   - per-session container lifecycle
#   - read-only workspace (inbound contracts)
#   - mutable workspace (redline output)
#   - MCP credentials: OAuth (CLM system), bearer (legal precedent database)
#   - inference: chat completions + embeddings (clause similarity search)
#   - event subscription: contract-received

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
LABEL org.openagentcontainers.name="contract-review-agent"

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
LABEL org.openagentcontainers.inference.chat-completions.input.vision="true"
LABEL org.openagentcontainers.inference.embeddings.context="2048"

# --- MCP: Contract lifecycle management system (OAuth) ---
LABEL org.openagentcontainers.mcp.clm-system.oauth.client_id.env="CLM_CLIENT_ID"
LABEL org.openagentcontainers.mcp.clm-system.oauth.client_secret.env="CLM_CLIENT_SECRET"

# --- MCP: Legal precedent database (bearer) ---
LABEL org.openagentcontainers.mcp.legal-precedents.bearer.token.env="LEGAL_DB_TOKEN"

# --- Workspaces ---
# Inbound contracts mounted read-only; redlines written to the output mount.
LABEL org.openagentcontainers.workspace.contracts.path="/workspace/contracts"
LABEL org.openagentcontainers.workspace.redlines.path="/workspace/redlines"
LABEL org.openagentcontainers.workspace.redlines.mutable="true"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.contract-received.schema.path="/oac/schemas/contract-received.json"
LABEL org.openagentcontainers.events.contract-received.schema.mimetype="application/schema+json"

COPY schemas/contract-received.json /oac/schemas/contract-received.json
COPY --from=builder /agent /app/agent

CMD ["/app/agent"]
