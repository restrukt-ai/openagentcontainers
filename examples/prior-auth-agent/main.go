// prior-auth-agent reviews medical prior authorization requests against
// clinical guidelines and payer coverage policies, then writes a structured
// approval or denial decision. One container per session — PHI requires
// infrastructure-level session isolation, not in-process demultiplexing.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

const (
	guidelinesPath = "/workspace/guidelines"
	decisionsPath  = "/workspace/decisions"
)

// AuthRequest is the event payload for auth-request events (schemas/auth-request.json).
type AuthRequest struct {
	RequestID       string   `json:"request_id"`
	PatientID       string   `json:"patient_id"`
	ProviderID      string   `json:"provider_id"`
	PayerID         string   `json:"payer_id,omitempty"`
	ProcedureCode   string   `json:"procedure_code"`
	DiagnosisCodes  []string `json:"diagnosis_codes"`
	ClinicalSummary string   `json:"clinical_summary,omitempty"`
	Priority        string   `json:"priority,omitempty"` // routine | urgent | emergent
	CreatedAt       string   `json:"created_at"`
}

// --- Tools ---

type ReadGuidelineArgs struct {
	ProcedureCode string `json:"procedure_code" jsonschema:"description=CPT or HCPCS code to look up clinical criteria for"`
}
type ReadGuidelineResult struct {
	Found     bool   `json:"found"`
	Guideline string `json:"guideline"` // full text of the clinical criteria
}

// readGuideline loads clinical criteria from the read-only guidelines workspace.
// The guidelines corpus is a directory of Markdown files named by procedure code.
func readGuideline(_ tool.Context, args ReadGuidelineArgs) (ReadGuidelineResult, error) {
	path := filepath.Join(guidelinesPath, args.ProcedureCode+".md")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ReadGuidelineResult{Found: false}, nil
	}
	if err != nil {
		return ReadGuidelineResult{}, fmt.Errorf("reading guideline for %s: %w", args.ProcedureCode, err)
	}
	return ReadGuidelineResult{Found: true, Guideline: string(b)}, nil
}

type WriteDecisionArgs struct {
	RequestID  string `json:"request_id"  jsonschema:"description=Authorization request ID"`
	Decision   string `json:"decision"    jsonschema:"description=approved or denied"`
	Rationale  string `json:"rationale"   jsonschema:"description=Clinical rationale for the decision"`
	Conditions string `json:"conditions"  jsonschema:"description=Any approval conditions or denial appeal instructions"`
}

// writeDecision persists the authorization decision to the mutable decisions workspace.
// The orchestrator makes this file available to downstream systems (EHR, payer portal).
func writeDecision(_ tool.Context, args WriteDecisionArgs) (map[string]any, error) {
	if args.Decision != "approved" && args.Decision != "denied" {
		return nil, fmt.Errorf("decision must be 'approved' or 'denied', got %q", args.Decision)
	}

	content := fmt.Sprintf("# Prior Authorization Decision\n\n"+
		"**Request ID:** %s\n"+
		"**Decision:** %s\n\n"+
		"## Clinical Rationale\n\n%s\n\n"+
		"## Conditions / Next Steps\n\n%s\n",
		args.RequestID, strings.ToUpper(args.Decision), args.Rationale, args.Conditions,
	)

	outPath := filepath.Join(decisionsPath, args.RequestID+".md")
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing decision: %w", err)
	}
	return map[string]any{"written": outPath, "decision": args.Decision}, nil
}

// --- Agent ---

func buildAgent(ctx context.Context) (agent.Agent, error) {
	// Orchestrator injects inference credentials via:
	//   org.openagentcontainers.inference.api_base.env="OPENAI_BASE_URL"
	//   org.openagentcontainers.inference.api_key.env="OPENAI_API_KEY"
	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	readGuidelineTool, _ := functiontool.New(functiontool.Config{
		Name:        "read_clinical_guideline",
		Description: "Read clinical necessity criteria for a CPT/HCPCS procedure code from the guidelines library.",
	}, readGuideline)

	writeDecisionTool, _ := functiontool.New(functiontool.Config{
		Name:        "write_auth_decision",
		Description: "Persist the authorization decision and rationale to the output workspace.",
	}, writeDecision)

	// EHR MCP server — FHIR-scoped DCR credentials injected by orchestrator (spec §5.3).
	ehrToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("ehr-mcp-server",
				"--client-id", os.Getenv("EHR_CLIENT_ID"),
				"--client-secret", os.Getenv("EHR_CLIENT_SECRET"),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating ehr toolset: %w", err)
	}

	// Payer rules MCP server — bearer token injected by orchestrator (spec §5.3).
	payerToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("payer-rules-mcp-server", "--token", os.Getenv("PAYER_RULES_TOKEN")),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating payer-rules toolset: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:  "prior-auth-agent",
		Model: model,
		Instruction: `You are a clinical prior authorization specialist. For each request:

1. Retrieve the patient's relevant medical history and current conditions from
   the EHR system (diagnoses, prior procedures, medications).
2. Read the clinical necessity criteria for the requested procedure using
   read_clinical_guideline.
3. Check the patient's coverage policy and any payer-specific criteria using
   the payer rules tools.
4. Assess whether the clinical evidence in the request meets medical necessity
   criteria. Consider diagnosis codes, clinical summary, and patient history.
5. Write a structured decision using write_auth_decision:
   - decision: "approved" or "denied"
   - rationale: cite specific criteria met or not met
   - conditions: approval conditions (e.g. step therapy required) or denial
     appeal instructions referencing the specific criteria not met

Be clinically precise. Every denial must cite the specific guideline criterion
that was not satisfied and explain the appeal pathway.`,
		Tools:    []tool.Tool{readGuidelineTool, writeDecisionTool},
		Toolsets: []tool.Toolset{ehrToolset, payerToolset},
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

	r, err := runner.New(runner.Config{
		AppName:           "prior-auth-agent",
		Agent:             ag,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Error("creating runner", "error", err)
		os.Exit(1)
	}

	runHarness(ctx, func(ctx context.Context, evt OACEvent) error {
		var req AuthRequest
		if err := json.Unmarshal(evt.Payload, &req); err != nil {
			return fmt.Errorf("decoding auth-request event: %w", err)
		}

		log.Info("reviewing auth request",
			"request_id", req.RequestID,
			"patient_id", req.PatientID,
			"procedure", req.ProcedureCode,
			"priority", req.Priority,
		)

		prompt := fmt.Sprintf(
			"Request ID: %s\nPatient ID: %s\nProvider ID: %s\n"+
				"Procedure: %s\nDiagnoses: %s\nPriority: %s\n\nClinical summary:\n%s",
			req.RequestID, req.PatientID, req.ProviderID,
			req.ProcedureCode, strings.Join(req.DiagnosisCodes, ", "),
			req.Priority, req.ClinicalSummary,
		)

		for event, err := range r.Run(ctx, "system", evt.SessionID,
			genai.NewUserContent(genai.Text(prompt)), agent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}
			if event.IsFinalResponse() {
				log.Info("decision written",
					"request_id", req.RequestID,
					"session_id", evt.SessionID,
				)
			}
		}
		return nil
	})
}
