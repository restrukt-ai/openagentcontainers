# Example: per-session coding agent
#
# Handles code tasks triggered by events: PR reviews, bug fixes, and feature
# implementations. Each task runs in a dedicated container with a private
# workspace clone of the target repository — the orchestrator provisions and
# tears down the workspace mount per session.
#
# Demonstrates:
#   - per-session container lifecycle (session.isolation absent)
#   - mutable workspace (code repository clone)
#   - read-only workspace (engineering docs reference)
#   - MCP credentials: DCR (GitHub), bearer (CI/CD pipeline)
#   - inference: chat completions + embeddings
#   - event subscription: code-task

FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /agent .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git grep

# --- Identity ---
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="coding-agent"
LABEL org.openagentcontainers.description="Handles code tasks (PR reviews, bug fixes, feature implementations) with per-session workspace isolation"

# --- Orchestrator connection (mTLS) ---
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_ADDR"
LABEL org.openagentcontainers.orchestrator.mtls.cert.file="/run/secrets/harness.crt"
LABEL org.openagentcontainers.orchestrator.mtls.key.file="/run/secrets/harness.key"
LABEL org.openagentcontainers.orchestrator.mtls.ca.file="/run/secrets/ca.crt"

# --- Inference ---
LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.models="gemini-2.5-flash"
LABEL org.openagentcontainers.inference.embeddings.models="text-embedding-004"

# --- MCP: GitHub (DCR) ---
LABEL org.openagentcontainers.mcp.github.dcr.scopes="repo:read repo:write pull-request:read pull-request:write checks:read checks:write"
LABEL org.openagentcontainers.mcp.github.dcr.client_id.env="GITHUB_CLIENT_ID"
LABEL org.openagentcontainers.mcp.github.dcr.client_secret.env="GITHUB_CLIENT_SECRET"

# --- MCP: CI/CD pipeline (bearer) ---
LABEL org.openagentcontainers.mcp.ci.bearer.token.env="CI_TOKEN"

# --- Workspaces ---
# The orchestrator provisions a thin clone of the target repository per session.
LABEL org.openagentcontainers.workspace.repo.path="/workspace/repo"
LABEL org.openagentcontainers.workspace.repo.mutable="true"
LABEL org.openagentcontainers.workspace.docs.path="/workspace/docs"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.code-task.schema.path="/oac/schemas/code-task.json"
LABEL org.openagentcontainers.events.code-task.schema.mimetype="application/schema+json"

COPY schemas/code-task.json /oac/schemas/code-task.json
COPY --from=builder /agent /app/agent

CMD ["/app/agent"]
