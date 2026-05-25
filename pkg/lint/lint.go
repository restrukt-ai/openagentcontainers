// Package lint provides validation checks for OAC manifests beyond what Parse/Validate enforce.
package lint

import (
	"sort"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// Severity indicates the severity of a lint issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue represents a single lint finding.
type Issue struct {
	Severity Severity `json:"severity"`
	Field    string   `json:"field"`
	Message  string   `json:"message"`
}

// Lint runs lint checks against an already-parsed manifest.
// Returns nil when there are no issues.
func Lint(m *oac.Manifest) []Issue {
	var issues []Issue

	var spec *oac.V1Alpha1Spec

	switch {
	case m.V1Alpha1 != nil:
		spec = m.V1Alpha1
	case m.V1Alpha2 != nil:
		spec = &m.V1Alpha2.V1Alpha1Spec
	default:
		return nil
	}

	lintSpec(spec, &issues)

	return issues
}

func lintSpec(spec *oac.V1Alpha1Spec, issues *[]Issue) {
	checkDescription(spec.Description, issues)
	checkInference(spec.Inference, issues)
	checkMCPs(spec.MCP, issues)
	checkOrchestrator(spec.Orchestrator, issues)
	checkWorkspaces(spec.Workspace, issues)
	checkEvents(spec.Events, issues)
}

func checkDescription(desc string, issues *[]Issue) {
	if desc == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "description",
			Message:  "description is not set",
		})
	}
}

func checkInference(inf *oac.InferenceSpec, issues *[]Issue) {
	if inf == nil {
		return
	}

	if inf.APIBase != nil && inf.APIKey == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "inference.api_key",
			Message:  "api_key is required when api_base is set",
		})
	}

	if inf.APIKey != nil && inf.APIBase == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "inference.api_base",
			Message:  "api_base is required when api_key is set",
		})
	}

	if inf.APIBase != nil {
		checkEnvFile("inference.api_base", *inf.APIBase, issues)
	}

	if inf.APIKey != nil {
		checkEnvFile("inference.api_key", *inf.APIKey, issues)
	}

	if len(inf.Types) == 0 && (inf.APIBase != nil || inf.APIKey != nil) {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "inference",
			Message:  "no inference types declared",
		})
	}
}

func checkMCPs(mcps map[string]oac.MCPSpec, issues *[]Issue) {
	keys := make([]string, 0, len(mcps))
	for k := range mcps {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, name := range keys {
		checkMCP(name, mcps[name], issues)
	}
}

func checkMCP(name string, m oac.MCPSpec, issues *[]Issue) {
	if m.Bearer == nil && m.OAuth == nil && m.DCR == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "mcp." + name,
			Message:  "no auth method configured",
		})

		return
	}

	if m.Bearer != nil {
		checkEnvFile("mcp."+name+".bearer.token", m.Bearer.Token, issues)
	}

	if m.OAuth != nil {
		checkEnvFile("mcp."+name+".oauth.client_id", m.OAuth.ClientID, issues)
		checkEnvFile("mcp."+name+".oauth.client_secret", m.OAuth.ClientSecret, issues)
	}

	if m.DCR != nil {
		if m.DCR.Scopes == "" {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				Field:    "mcp." + name + ".dcr.scopes",
				Message:  "no scopes declared",
			})
		}

		checkEnvFile("mcp."+name+".dcr.client_id", m.DCR.ClientID, issues)
		checkEnvFile("mcp."+name+".dcr.client_secret", m.DCR.ClientSecret, issues)
	}
}

func checkOrchestrator(orch *oac.OrchestratorSpec, issues *[]Issue) {
	if orch == nil {
		return
	}

	if orch.Env == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "orchestrator.env",
			Message:  "env is not set",
		})
	}

	if orch.Bearer == nil && orch.MTLS == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "orchestrator",
			Message:  "no auth method configured",
		})

		return
	}

	if orch.Bearer != nil {
		checkEnvFile("orchestrator.bearer.token", orch.Bearer.Token, issues)
	}

	if orch.MTLS != nil {
		checkEnvFile("orchestrator.mtls.cert", orch.MTLS.Cert, issues)
		checkEnvFile("orchestrator.mtls.key", orch.MTLS.Key, issues)
		// CA is optional; the orchestrator provisions it
	}
}

func checkWorkspaces(ws map[string]oac.WorkspaceSpec, issues *[]Issue) {
	keys := make([]string, 0, len(ws))
	for k := range ws {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, name := range keys {
		if ws[name].Path == "" {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				Field:    "workspace." + name + ".path",
				Message:  "path is empty",
			})
		}
	}
}

func checkEvents(evs map[string]oac.EventSpec, issues *[]Issue) {
	keys := make([]string, 0, len(evs))
	for k := range evs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, name := range keys {
		ev := evs[name]

		if ev.Schema.Path == "" {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				Field:    "events." + name + ".schema.path",
				Message:  "schema path is not set",
			})
		}

		if ev.Schema.MIMEType == "" {
			*issues = append(*issues, Issue{
				Severity: SeverityWarning,
				Field:    "events." + name + ".schema.mimetype",
				Message:  "schema mimetype is not set",
			})
		}
	}
}

func checkEnvFile(field string, ef oac.EnvFile, issues *[]Issue) {
	if ef.Env == "" && ef.File == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Field:    field,
			Message:  "neither env nor file set; no credential source",
		})
	}
	// both set is valid per spec §5.3
}
