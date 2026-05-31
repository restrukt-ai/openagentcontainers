# Example: session-isolated trade surveillance agent
#
# Screens every executed trade for regulatory violations: wash trading,
# front-running, spoofing, and position limit breaches. Flags suspicious
# activity for compliance review and files SARs when thresholds are exceeded.
# Session-isolated — high trade volume requires horizontal scaling behind a
# load balancer, not one container per trade.
#
# Demonstrates:
#   - session isolation (extreme-throughput event processing)
#   - MCP credentials: bearer (real-time market data), DCR (compliance rules)
#   - inference: chat completions + embeddings (pattern detection)
#   - event subscription: trade-executed

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
LABEL org.openagentcontainers.name="trade-surveillance-agent"

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

# --- MCP: Real-time market data feed (bearer) ---
LABEL org.openagentcontainers.mcp.market-data.bearer.token.env="MARKET_DATA_TOKEN"

# --- MCP: Compliance rules engine (DCR) ---
LABEL org.openagentcontainers.mcp.compliance-rules.dcr.scopes="rules:read positions:read orders:read alerts:write sar:write"
LABEL org.openagentcontainers.mcp.compliance-rules.dcr.client_id.env="COMPLIANCE_CLIENT_ID"
LABEL org.openagentcontainers.mcp.compliance-rules.dcr.client_secret.env="COMPLIANCE_CLIENT_SECRET"

# --- Session isolation ---
LABEL org.openagentcontainers.session.isolation="true"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.trade-executed.schema.path="/oac/schemas/trade-executed.json"
LABEL org.openagentcontainers.events.trade-executed.schema.mimetype="application/schema+json"

COPY schemas/trade-executed.json /oac/schemas/trade-executed.json
COPY --from=builder /agent /app/agent

CMD ["/app/agent"]
