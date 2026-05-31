# Example: session-isolated support agent
#
# This agent handles inbound support tickets. It is session-isolated — a single
# process manages many concurrent support conversations — so the orchestrator
# deploys it as a long-running service behind a load balancer rather than
# starting a new container per session.
#
# Demonstrates:
#   - orchestrator connection via mTLS
#   - inference (chat completions + embeddings)
#   - MCP credentials: DCR (CRM) and bearer token (knowledge base)
#   - event subscription (inbound support tickets)
#   - session isolation

FROM python:3.12-slim

# --- Identity ---
LABEL org.openagentcontainers.version="v1alpha3"
LABEL org.openagentcontainers.name="support-agent"

# --- Orchestrator connection ---
# The harness connects to the orchestrator at startup via mTLS.
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_ADDR"
LABEL org.openagentcontainers.orchestrator.mtls.cert.file="/run/secrets/harness.crt"
LABEL org.openagentcontainers.orchestrator.mtls.key.file="/run/secrets/harness.key"
LABEL org.openagentcontainers.orchestrator.mtls.ca.file="/run/secrets/ca.crt"

# --- Inference ---
LABEL org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
LABEL org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
LABEL org.openagentcontainers.inference.chat-completions.tools="true"
LABEL org.openagentcontainers.inference.chat-completions.context="128000"
LABEL org.openagentcontainers.inference.embeddings.context="8191"

# --- MCP credentials ---
LABEL org.openagentcontainers.mcp.crm.dcr.scopes="tickets:read tickets:write contacts:read"
LABEL org.openagentcontainers.mcp.crm.dcr.client_id.env="CRM_CLIENT_ID"
LABEL org.openagentcontainers.mcp.crm.dcr.client_secret.env="CRM_CLIENT_SECRET"

# Knowledge base server: static bearer token injected by the orchestrator.
LABEL org.openagentcontainers.mcp.knowledge-base.bearer.token.env="KB_TOKEN"

# --- Event subscriptions ---
LABEL org.openagentcontainers.events.support-ticket.schema.path="/oac/schemas/support-ticket.json"
LABEL org.openagentcontainers.events.support-ticket.schema.mimetype="application/schema+json"

# --- Session isolation ---
LABEL org.openagentcontainers.session.isolation="true"

COPY schemas/support-ticket.json /oac/schemas/support-ticket.json
COPY agent.py /app/agent.py

CMD ["python", "/app/agent.py"]
