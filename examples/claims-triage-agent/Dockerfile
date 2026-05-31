# Example: session-isolated insurance claims triage agent
#
# Triages inbound insurance claims: classifies incident type, assesses fraud
# risk, requests missing documentation, and routes to the appropriate adjuster.
# Session-isolated — a single process handles many concurrent claims — so the
# orchestrator deploys it as a long-running service behind a load balancer.
#
# Demonstrates:
#   - session isolation (high-concurrency stateless routing)
#   - MCP credentials: DCR (claims management system), bearer (fraud database)
#   - inference: chat completions + embeddings (fraud pattern matching)
#             + audio transcriptions (recorded intake calls)
#   - event subscription: insurance-claim

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
LABEL org.openagentcontainers.name="claims-triage-agent"

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
LABEL org.openagentcontainers.inference.audio-transcriptions.input.audio="true"

# --- MCP: Claims management system (DCR) ---
LABEL org.openagentcontainers.mcp.claims-system.dcr.scopes="claims:read claims:write policies:read customers:read adjusters:read"
LABEL org.openagentcontainers.mcp.claims-system.dcr.client_id.env="CLAIMS_CLIENT_ID"
LABEL org.openagentcontainers.mcp.claims-system.dcr.client_secret.env="CLAIMS_CLIENT_SECRET"

# --- Session isolation ---
LABEL org.openagentcontainers.session.isolation="true"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.insurance-claim.schema.path="/oac/schemas/insurance-claim.json"
LABEL org.openagentcontainers.events.insurance-claim.schema.mimetype="application/schema+json"

COPY schemas/insurance-claim.json /oac/schemas/insurance-claim.json
COPY --from=builder /agent /app/agent

CMD ["/app/agent"]
