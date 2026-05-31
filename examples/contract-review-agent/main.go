// contract-review-agent reviews inbound contracts against company standard
// templates, flags non-standard and high-risk clauses, and produces a
// tracked-changes redline document ready for legal sign-off.
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
	contractsPath = "/workspace/contracts"
	redlinesPath  = "/workspace/redlines"
)

// ContractReceived is the event payload for contract-received events
// (schemas/contract-received.json).
type ContractReceived struct {
	ContractID         string `json:"contract_id"`
	ContractType       string `json:"contract_type"` // nda | msa | sow | saas | ...
	Counterparty       string `json:"counterparty"`
	CounterpartyID     string `json:"counterparty_id,omitempty"`
	DocumentPath       string `json:"document_path"`
	ReviewInstructions string `json:"review_instructions,omitempty"`
	ReviewDeadline     string `json:"review_deadline,omitempty"`
	CreatedAt          string `json:"created_at"`
}

// --- Tools ---

type ReadContractArgs struct {
	DocumentPath string `json:"document_path" jsonschema:"description=Path within /workspace/contracts to read"`
}
type ReadContractResult struct {
	Content string `json:"content"`
	Bytes   int    `json:"bytes"`
}

func readContract(_ tool.Context, args ReadContractArgs) (ReadContractResult, error) {
	// Sanitize: only allow reads within the contracts mount.
	path := filepath.Join(contractsPath, filepath.Clean("/"+args.DocumentPath))
	b, err := os.ReadFile(path)
	if err != nil {
		return ReadContractResult{}, fmt.Errorf("reading contract at %s: %w", path, err)
	}
	return ReadContractResult{Content: string(b), Bytes: len(b)}, nil
}

type RiskClause struct {
	Section     string `json:"section"`
	ClauseText  string `json:"clause_text"`
	RiskType    string `json:"risk_type"`    // liability | ip | termination | payment | other
	Severity    string `json:"severity"`     // low | medium | high
	Explanation string `json:"explanation"`
}

type IdentifyRisksArgs struct {
	ContractText string `json:"contract_text" jsonschema:"description=Full contract text to analyze for risk clauses"`
	ContractType string `json:"contract_type" jsonschema:"description=Contract category (nda, msa, sow, etc.)"`
}
type IdentifyRisksResult struct {
	Clauses     []RiskClause `json:"clauses"`
	TotalHigh   int          `json:"total_high"`
	TotalMedium int          `json:"total_medium"`
	Summary     string       `json:"summary"`
}

// identifyRisks extracts and classifies risk clauses from contract text.
// The LLM does the semantic heavy lifting; this function structures its output.
func identifyRisks(_ tool.Context, args IdentifyRisksArgs) (IdentifyRisksResult, error) {
	// In production: call an embeddings-based clause classifier here.
	// This stub returns a placeholder so the agent can demonstrate the workflow.
	clauses := []RiskClause{
		{
			Section:     "§8.2",
			ClauseText:  "Vendor shall not be liable for indirect, incidental, or consequential damages.",
			RiskType:    "liability",
			Severity:    "high",
			Explanation: "Broad liability cap excludes consequential damages. Standard MSA allows up to 2× annual fees.",
		},
		{
			Section:     "§12.1",
			ClauseText:  "Either party may terminate with 30 days written notice.",
			RiskType:    "termination",
			Severity:    "medium",
			Explanation: "30-day termination window is shorter than standard 90-day. May leave insufficient migration time.",
		},
	}

	high := 0
	for _, c := range clauses {
		if c.Severity == "high" {
			high++
		}
	}

	return IdentifyRisksResult{
		Clauses:     clauses,
		TotalHigh:   high,
		TotalMedium: len(clauses) - high,
		Summary:     fmt.Sprintf("%d high-risk and %d medium-risk clauses identified", high, len(clauses)-high),
	}, nil
}

type WriteRedlineArgs struct {
	ContractID  string `json:"contract_id"  jsonschema:"description=Contract identifier, used as the output filename"`
	RedlineText string `json:"redline_text" jsonschema:"description=Full contract text with tracked changes in markdown diff format"`
	Summary     string `json:"summary"      jsonschema:"description=Executive summary of findings for the legal team"`
}

func writeRedline(_ tool.Context, args WriteRedlineArgs) (map[string]any, error) {
	content := fmt.Sprintf("# Contract Review: %s\n\n## Executive Summary\n\n%s\n\n## Redline\n\n%s",
		args.ContractID, args.Summary, args.RedlineText)

	outPath := filepath.Join(redlinesPath, args.ContractID+"-redline.md")
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing redline: %w", err)
	}
	return map[string]any{"written": outPath}, nil
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

	readContractTool, _ := functiontool.New(functiontool.Config{
		Name:        "read_contract",
		Description: "Read the inbound contract document from the contracts workspace.",
	}, readContract)

	identifyRisksTool, _ := functiontool.New(functiontool.Config{
		Name:        "identify_risk_clauses",
		Description: "Extract and classify non-standard or high-risk clauses from contract text.",
	}, identifyRisks)

	writeRedlineTool, _ := functiontool.New(functiontool.Config{
		Name:        "write_redline",
		Description: "Write the reviewed contract with tracked changes and executive summary to the redlines workspace.",
	}, writeRedline)

	// CLM system MCP server — OAuth credentials injected by orchestrator (spec §5.3).
	clmToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("clm-mcp-server",
				"--client-id", os.Getenv("CLM_CLIENT_ID"),
				"--client-secret", os.Getenv("CLM_CLIENT_SECRET"),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating clm toolset: %w", err)
	}

	// Legal precedent database MCP server — bearer token injected by orchestrator (spec §5.3).
	legalToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{
			Command: exec.Command("legal-db-mcp-server", "--token", os.Getenv("LEGAL_DB_TOKEN")),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating legal-db toolset: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:  "contract-review-agent",
		Model: model,
		Instruction: `You are a senior commercial contracts attorney reviewing inbound contracts.

For each contract:
1. Read the full contract text using read_contract.
2. Retrieve the company's standard template for this contract type from the CLM system.
3. Run identify_risk_clauses to extract non-standard terms.
4. Search the legal precedent database for how similar clauses have been handled
   in past negotiations.
5. Produce a redline using write_redline that:
   - Marks deviations from standard terms using markdown diff format (+ added, - removed)
   - Annotates each high-risk clause with the suggested replacement and legal rationale
   - Includes an executive summary listing: overall risk level, top 3 concerns,
     and recommended negotiation positions

Flag any clause that creates unlimited liability, perpetual IP assignment, or
unilateral termination rights as HIGH RISK requiring senior counsel review.`,
		Tools:    []tool.Tool{readContractTool, identifyRisksTool, writeRedlineTool},
		Toolsets: []tool.Toolset{clmToolset, legalToolset},
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
		AppName:           "contract-review-agent",
		Agent:             ag,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		log.Error("creating runner", "error", err)
		os.Exit(1)
	}

	runHarness(ctx, func(ctx context.Context, evt OACEvent) error {
		var contract ContractReceived
		if err := json.Unmarshal(evt.Payload, &contract); err != nil {
			return fmt.Errorf("decoding contract-received event: %w", err)
		}

		log.Info("reviewing contract",
			"contract_id", contract.ContractID,
			"type", contract.ContractType,
			"counterparty", contract.Counterparty,
		)

		parts := []string{
			fmt.Sprintf("Contract ID: %s", contract.ContractID),
			fmt.Sprintf("Type: %s", contract.ContractType),
			fmt.Sprintf("Counterparty: %s", contract.Counterparty),
			fmt.Sprintf("Document: %s", contract.DocumentPath),
		}
		if contract.ReviewDeadline != "" {
			parts = append(parts, "Deadline: "+contract.ReviewDeadline)
		}
		if contract.ReviewInstructions != "" {
			parts = append(parts, "\nInstructions:\n"+contract.ReviewInstructions)
		}

		for event, err := range r.Run(ctx, "system", evt.SessionID,
			genai.NewUserContent(genai.Text(strings.Join(parts, "\n"))), agent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}
			if event.IsFinalResponse() {
				log.Info("review complete",
					"contract_id", contract.ContractID,
					"session_id", evt.SessionID,
				)
			}
		}
		return nil
	})
}
