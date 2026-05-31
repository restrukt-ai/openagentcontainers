// claims-triage-agent triages inbound insurance claims: assesses fraud risk,
// classifies incident type, requests missing documentation, and routes to the
// appropriate adjuster. Session-isolated to handle high claim volume concurrently.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/exec"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// InsuranceClaim is the event payload for insurance-claim events (schemas/insurance-claim.json).
type InsuranceClaim struct {
	ClaimID         string   `json:"claim_id"`
	PolicyNumber    string   `json:"policy_number"`
	ClaimantID      string   `json:"claimant_id"`
	IncidentType    string   `json:"incident_type"`
	IncidentDate    string   `json:"incident_date,omitempty"`
	ClaimedAmount   float64  `json:"claimed_amount"`
	Description     string   `json:"description,omitempty"`
	RecordedCallURL string   `json:"recorded_call_url,omitempty"`
	Attachments     []string `json:"attachments,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

// --- Tools ---

type FraudCheckArgs struct {
	ClaimantID    string  `json:"claimant_id"    jsonschema:"description=Claimant identifier to look up in the fraud database"`
	ClaimedAmount float64 `json:"claimed_amount" jsonschema:"description=Dollar amount being claimed"`
	IncidentType  string  `json:"incident_type"  jsonschema:"description=Category of the reported incident"`
}
type FraudCheckResult struct {
	RiskScore    float64  `json:"risk_score"`    // 0.0–1.0
	RiskLevel    string   `json:"risk_level"`    // low | medium | high
	Indicators   []string `json:"indicators"`    // specific risk signals found
	PriorClaims  int      `json:"prior_claims"`  // count of prior claims by this claimant
}

// checkFraudRisk scores the claim against the fraud database. In production
// this logic lives inside the fraud-db MCP server; this function handles
// local pre-screening before the MCP call.
func checkFraudRisk(_ tool.Context, args FraudCheckArgs) (FraudCheckResult, error) {
	var indicators []string
	score := 0.0

	if args.ClaimedAmount > 50_000 {
		score += 0.3
		indicators = append(indicators, "high claimed amount")
	}
	if args.IncidentType == "auto-theft" {
		score += 0.15
		indicators = append(indicators, "theft incident type")
	}

	// Simulate prior claim lookup; real logic hits fraud-db via MCP.
	priorClaims := rand.Intn(3)
	if priorClaims > 1 {
		score += 0.25
		indicators = append(indicators, fmt.Sprintf("%d prior claims in 24 months", priorClaims))
	}

	level := "low"
	if score >= 0.5 {
		level = "high"
	} else if score >= 0.25 {
		level = "medium"
	}

	return FraudCheckResult{
		RiskScore:   score,
		RiskLevel:   level,
		Indicators:  indicators,
		PriorClaims: priorClaims,
	}, nil
}

type ClassifyArgs struct {
	IncidentType    string  `json:"incident_type"    jsonschema:"description=Incident type from the claim event"`
	ClaimedAmount   float64 `json:"claimed_amount"   jsonschema:"description=Dollar amount being claimed"`
	FraudRiskLevel  string  `json:"fraud_risk_level" jsonschema:"description=Fraud risk level from checkFraudRisk"`
}
type ClassifyResult struct {
	AdjusterType string `json:"adjuster_type"` // field | desk | special-investigations
	Priority     string `json:"priority"`      // routine | expedited | urgent
	Notes        string `json:"notes"`
}

func classifyClaim(_ tool.Context, args ClassifyArgs) (ClassifyResult, error) {
	adjusterType := "desk"
	priority := "routine"
	notes := ""

	if args.FraudRiskLevel == "high" {
		adjusterType = "special-investigations"
		priority = "expedited"
		notes = "Routed to SIU due to elevated fraud risk score."
	} else if args.ClaimedAmount > 25_000 || args.IncidentType == "liability" {
		adjusterType = "field"
		priority = "expedited"
		notes = "Field adjuster required for on-site assessment."
	}

	return ClassifyResult{
		AdjusterType: adjusterType,
		Priority:     priority,
		Notes:        notes,
	}, nil
}

// --- Agent ---

func buildAgent(ctx context.Context) (agent.Agent, error) {
	// Orchestrator injects inference credentials via:
	//   org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
	//   org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
	model, err := gemini.NewModel(ctx, "gemini-2.0-flash", &genai.ClientConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	fraudCheckTool, _ := functiontool.New(functiontool.Config{
		Name:        "check_fraud_risk",
		Description: "Pre-screen a claim for fraud risk indicators. Returns a risk score and list of signals.",
	}, checkFraudRisk)

	classifyTool, _ := functiontool.New(functiontool.Config{
		Name:        "classify_claim",
		Description: "Determine adjuster type and routing priority based on claim characteristics.",
	}, classifyClaim)

	// Claims management MCP server — DCR credentials injected by orchestrator (spec §5.3).
	claimsToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("claims-mcp-server",
				"--client-id", os.Getenv("CLAIMS_CLIENT_ID"),
				"--client-secret", os.Getenv("CLAIMS_CLIENT_SECRET"),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating claims toolset: %w", err)
	}

	// Fraud database MCP server — bearer token injected by orchestrator (spec §5.3).
	fraudToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("fraud-db-mcp-server", "--token", os.Getenv("FRAUD_DB_TOKEN")),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating fraud-db toolset: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:  "claims-triage-agent",
		Model: model,
		Instruction: `You are an insurance claims triage specialist. For each incoming claim:

1. Look up the customer's policy and claim history using the claims system tools.
2. Run check_fraud_risk to score the claim for fraud indicators.
3. If a recorded intake call URL is provided, request a transcription via the
   claims system tools and factor it into your assessment.
4. Run classify_claim to determine adjuster type and routing priority.
5. Update the claim record with your triage assessment using the claims system tools,
   including: fraud risk score, adjuster type, priority, and a brief rationale.
6. If fraud risk is high, flag the claim for Special Investigations Unit review.

Be thorough but concise. Claimants should not wait more than a few minutes for
initial routing.`,
		Tools:    []tool.Tool{fraudCheckTool, classifyTool},
		Toolsets: []tool.Toolset{claimsToolset, fraudToolset},
	})
}

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	ag, err := buildAgent(ctx)
	if err != nil {
		log.Error("building agent", "error", err)
		os.Exit(1)
	}

	// Session-isolated: a single runner handles all concurrent claims.
	// The orchestrator routes each claim to this process via session_id (spec §5.7).
	r, err := runner.New(runner.Config{
		AppName:           "claims-triage-agent",
		Agent:             ag,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Error("creating runner", "error", err)
		os.Exit(1)
	}

	runHarness(ctx, func(ctx context.Context, evt OACEvent) error {
		var claim InsuranceClaim
		if err := json.Unmarshal(evt.Payload, &claim); err != nil {
			return fmt.Errorf("decoding insurance-claim event: %w", err)
		}

		log.Info("triaging claim",
			"claim_id", claim.ClaimID,
			"incident_type", claim.IncidentType,
			"claimed_amount", claim.ClaimedAmount,
			"session_id", evt.SessionID,
		)

		prompt := fmt.Sprintf(
			"Claim ID: %s\nPolicy: %s\nClaimant ID: %s\nIncident type: %s\nIncident date: %s\n"+
				"Claimed amount: $%.2f\nDescription: %s",
			claim.ClaimID, claim.PolicyNumber, claim.ClaimantID,
			claim.IncidentType, claim.IncidentDate,
			claim.ClaimedAmount, claim.Description,
		)
		if claim.RecordedCallURL != "" {
			prompt += "\nRecorded call available: " + claim.RecordedCallURL
		}

		for event, err := range r.Run(ctx, "system", evt.SessionID,
			genai.NewUserContent(genai.Text(prompt)), agent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}
			if event.IsFinalResponse() {
				log.Info("triage complete", "claim_id", claim.ClaimID, "session_id", evt.SessionID)
			}
		}
		return nil
	})
}
