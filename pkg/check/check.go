// Package check provides checks for OAC manifests.
//
// Call [oac.Parse] first; [Check] performs all validation and advisory checks in a single pass.
// Each [Issue] carries a [Severity]: [SeverityError] means a required field or credential source is
// absent; [SeverityWarning] means a best-practice recommendation was not met.
//
// Checks performed:
//   - name: errors when name is not set
//   - description: warns when no description is set
//   - inference: warns when api_base/api_key are not declared together;
//     warns when credential env/file sources are empty
//   - mcp.<name>: warns when no auth method is configured; errors when
//     bearer/oauth/dcr credential sources are empty; warns when DCR scopes are absent
//   - orchestrator: errors when orchestrator is nil, env is missing, or auth is missing;
//     errors when credential sources are empty (orchestrator.mtls.ca is optional)
//   - session.isolation: errors when combined with workspaces (v1alpha2 only)
//   - workspace.<name>: warns when path is empty
//   - events.<name>: warns when schema path or mimetype is empty
package check

import (
	"slices"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// Severity indicates the severity of a lint issue.
// SeverityError indicates a structural problem: a required field or credential source (env or
// file) is missing and the image cannot function correctly. SeverityWarning indicates a
// best-practice issue; the image may still work but is likely misconfigured.
type Severity string

const (
	// SeverityError means a required field or credential source is absent;
	// the image will likely fail to start correctly.
	SeverityError Severity = "error"

	// SeverityWarning means a best-practice recommendation was not followed;
	// the image may still work but is likely misconfigured.
	SeverityWarning Severity = "warning"
)

// Issue represents a single lint finding.
// Field uses dot-notation matching the OAC label key suffix,
// e.g. "inference.api_base", "mcp.calendar.bearer.token", "orchestrator.mtls.cert",
// "workspace.data.path", "events.order-placed.schema.path".
type Issue struct {
	// Severity is the level of this finding ([SeverityError] or [SeverityWarning]).
	Severity Severity `json:"severity"`
	Field    string   `json:"field"`
	// Message is a human-readable description of the issue.
	Message string `json:"message"`
}

// Check runs all validation and advisory checks against an already-parsed manifest.
// Call Check after Parse. Returns nil when no issues are found.
// m must not be nil. If m has no populated spec (both V1Alpha1 and V1Alpha2 are nil),
// Check returns nil without panicking. See the package documentation for the full list
// of checks and their severities.
func Check(m *oac.Manifest) []Issue {
	var issues []Issue

	var spec *oac.V1Alpha1Spec

	switch {
	case m.V1Alpha1 != nil:
		spec = m.V1Alpha1
	case m.V1Alpha2 != nil:
		spec = &m.V1Alpha2.V1Alpha1Spec
		if m.V1Alpha2.Session.Isolation && len(m.V1Alpha2.Workspace) > 0 {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Field:    "session.isolation",
				Message:  "session.isolation cannot be combined with workspaces",
			})
		}
	default:
		return nil
	}

	checkSpec(spec, &issues)

	return issues
}

func checkSpec(spec *oac.V1Alpha1Spec, issues *[]Issue) {
	checkName(spec.Name, issues)
	checkDescription(spec.Description, issues)
	checkInference(spec.Inference, issues)
	checkMCPs(spec.MCP, issues)
	checkOrchestrator(spec.Orchestrator, issues)
	checkWorkspaces(spec.Workspace, issues)
	checkEvents(spec.Events, issues)
}

func checkName(name string, issues *[]Issue) {
	if name == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Field:    "name",
			Message:  "name is required",
		})
	}
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

	checkInferenceAPIKeyPair(inf, issues)
	checkInferenceEnvFiles(inf, issues)

	if len(inf.Types) == 0 && (inf.APIBase != nil || inf.APIKey != nil) {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Field:    "inference",
			Message:  "no inference types declared",
		})
	}
}

// checkInferenceAPIKeyPair warns when api_base and api_key are not set together.
func checkInferenceAPIKeyPair(inf *oac.InferenceSpec, issues *[]Issue) {
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
}

// checkInferenceEnvFiles validates credential sources for api_base and api_key.
func checkInferenceEnvFiles(inf *oac.InferenceSpec, issues *[]Issue) {
	if inf.APIBase != nil {
		checkEnvFile("inference.api_base", *inf.APIBase, issues)
	}

	if inf.APIKey != nil {
		checkEnvFile("inference.api_key", *inf.APIKey, issues)
	}
}

func checkMCPs(mcps map[string]oac.MCPSpec, issues *[]Issue) {
	keys := make([]string, 0, len(mcps))
	for k := range mcps {
		keys = append(keys, k)
	}

	slices.Sort(keys)

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
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Field:    "orchestrator",
			Message:  "orchestrator is required",
		})

		return
	}

	if orch.Env == "" {
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Field:    "orchestrator.env",
			Message:  "env is not set",
		})
	}

	if orch.Bearer == nil && orch.MTLS == nil {
		*issues = append(*issues, Issue{
			Severity: SeverityError,
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

	slices.Sort(keys)

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

	slices.Sort(keys)

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
