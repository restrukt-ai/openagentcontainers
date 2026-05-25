package lint_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/lint"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func mustParse(t *testing.T, labels map[string]string) *oac.Manifest {
	t.Helper()

	m, err := oac.Parse(labels)
	require.NoError(t, err)

	return m
}

func baseLabels() map[string]string {
	return map[string]string{
		"org.openagentcontainers.version": "v1alpha2",
		"org.openagentcontainers.name":    "test-agent",
	}
}

func merge(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	maps.Copy(out, base)

	maps.Copy(out, extra)

	return out
}

func findIssue(issues []lint.Issue, field string) *lint.Issue {
	for i := range issues {
		if issues[i].Field == field {
			return &issues[i]
		}
	}

	return nil
}

func TestLint_Clean(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.description":                       "A clean agent",
		"org.openagentcontainers.inference.api_base.env":            "OPENAI_BASE_URL",
		"org.openagentcontainers.inference.api_key.env":             "OPENAI_API_KEY",
		"org.openagentcontainers.inference.chat-completions.models": "gpt-4o",
		"org.openagentcontainers.orchestrator.env":                  "ORCHESTRATOR_ADDR",
		"org.openagentcontainers.orchestrator.bearer.token.env":     "ORCH_TOKEN",
	})

	issues := lint.Lint(mustParse(t, labels))
	assert.Empty(t, issues)
}

func TestLint_DescriptionEmpty(t *testing.T) {
	t.Parallel()

	issues := lint.Lint(mustParse(t, baseLabels()))

	iss := findIssue(issues, "description")
	require.NotNil(t, iss, "expected description warning")
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_InferenceAPIBaseWithoutAPIKey(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.inference.api_base.env": "OPENAI_BASE_URL",
	})

	issues := lint.Lint(mustParse(t, labels))

	iss := findIssue(issues, "inference.api_key")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_InferenceAPIKeyWithoutAPIBase(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.inference.api_key.env": "OPENAI_API_KEY",
	})

	issues := lint.Lint(mustParse(t, labels))

	iss := findIssue(issues, "inference.api_base")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_InferenceNoTypes(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.inference.api_base.env": "OPENAI_BASE_URL",
		"org.openagentcontainers.inference.api_key.env":  "OPENAI_API_KEY",
	})

	issues := lint.Lint(mustParse(t, labels))

	iss := findIssue(issues, "inference")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_InferenceClean(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.inference.api_base.env":            "OPENAI_BASE_URL",
		"org.openagentcontainers.inference.api_key.env":             "OPENAI_API_KEY",
		"org.openagentcontainers.inference.chat-completions.models": "gpt-4o",
	})

	issues := lint.Lint(mustParse(t, labels))

	assert.Nil(t, findIssue(issues, "inference"))
	assert.Nil(t, findIssue(issues, "inference.api_base"))
	assert.Nil(t, findIssue(issues, "inference.api_key"))
}

func TestLint_MCPNoAuthMethod(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {}, // all auth methods nil
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_MCPBearerTokenNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {
						Bearer: &oac.MCPBearerAuth{
							Token: oac.EnvFile{}, // neither env nor file
						},
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv.bearer.token")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
	assert.Contains(t, iss.Message, "no credential source")
}

func TestLint_MCPBearerBothEnvAndFile(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.mcp.srv.bearer.token.env":  "MY_TOKEN",
		"org.openagentcontainers.mcp.srv.bearer.token.file": "/run/secrets/token",
	})

	issues := lint.Lint(mustParse(t, labels))

	// both is valid per spec §5.3 — no issue on this field
	assert.Nil(t, findIssue(issues, "mcp.srv.bearer.token"))
}

func TestLint_MCPOAuthClientIDNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {
						OAuth: &oac.MCPOAuthAuth{
							ClientID:     oac.EnvFile{},              // no source
							ClientSecret: oac.EnvFile{Env: "SECRET"}, // has source
						},
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv.oauth.client_id")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_MCPOAuthClientSecretNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {
						OAuth: &oac.MCPOAuthAuth{
							ClientID:     oac.EnvFile{Env: "ID"}, // has source
							ClientSecret: oac.EnvFile{},          // no source
						},
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv.oauth.client_secret")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_MCPDCRNoScopes(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {
						DCR: &oac.MCPDCRAuth{
							Scopes:       "", // no scopes
							ClientID:     oac.EnvFile{Env: "ID"},
							ClientSecret: oac.EnvFile{Env: "SECRET"},
						},
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv.dcr.scopes")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_MCPDCRClientIDNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				MCP: map[string]oac.MCPSpec{
					"srv": {
						DCR: &oac.MCPDCRAuth{
							Scopes:       "repo:read",
							ClientID:     oac.EnvFile{},              // no source
							ClientSecret: oac.EnvFile{Env: "SECRET"}, // has source
						},
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "mcp.srv.dcr.client_id")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_OrchestratorEnvEmpty(t *testing.T) {
	t.Parallel()

	// Set bearer token but omit orchestrator.env label — Env will be ""
	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCH_TOKEN",
	})

	issues := lint.Lint(mustParse(t, labels))

	iss := findIssue(issues, "orchestrator.env")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_OrchestratorNoAuth(t *testing.T) {
	t.Parallel()

	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.orchestrator.env": "ORCHESTRATOR_ADDR",
	})

	issues := lint.Lint(mustParse(t, labels))

	iss := findIssue(issues, "orchestrator")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_OrchestratorBearerNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
					Bearer: &oac.OrchestratorBearerAuth{
						Token: oac.EnvFile{}, // no source
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "orchestrator.bearer.token")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_OrchestratorMTLSCertNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
					MTLS: &oac.OrchestratorMTLSAuth{
						Cert: oac.EnvFile{},             // no source
						Key:  oac.EnvFile{File: "/key"}, // has source
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "orchestrator.mtls.cert")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_OrchestratorMTLSKeyNoSource(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
					MTLS: &oac.OrchestratorMTLSAuth{
						Cert: oac.EnvFile{File: "/cert"}, // has source
						Key:  oac.EnvFile{},              // no source
					},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "orchestrator.mtls.key")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityError, iss.Severity)
}

func TestLint_WorkspacePathEmpty(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Workspace: map[string]oac.WorkspaceSpec{
					"code": {Path: ""},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "workspace.code.path")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_EventSchemaPathEmpty(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Events: map[string]oac.EventSpec{
					"start": {Schema: oac.EventSchema{MIMEType: "application/schema+json"}},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "events.start.schema.path")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_EventSchemaMimetypeEmpty(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		SpecVersion: oac.VersionV1Alpha2,
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "test-agent",
				Events: map[string]oac.EventSpec{
					"start": {Schema: oac.EventSchema{Path: "/oac/schemas/start.json"}},
				},
			},
		},
	}

	issues := lint.Lint(m)

	iss := findIssue(issues, "events.start.schema.mimetype")
	require.NotNil(t, iss)
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_MultipleIssuesAdditive(t *testing.T) {
	t.Parallel()

	// no description, api_base without api_key, orchestrator with no auth
	labels := merge(baseLabels(), map[string]string{
		"org.openagentcontainers.inference.api_base.env": "OPENAI_BASE_URL",
		"org.openagentcontainers.orchestrator.env":       "ORCHESTRATOR_ADDR",
	})

	issues := lint.Lint(mustParse(t, labels))

	assert.Greater(t, len(issues), 1)
}

func TestLint_V1Alpha1Dispatch(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version": "v1alpha1",
		"org.openagentcontainers.name":    "v1agent",
	}

	m := mustParse(t, labels)
	require.NotNil(t, m.V1Alpha1)

	issues := lint.Lint(m)

	iss := findIssue(issues, "description")
	require.NotNil(t, iss, "expected description warning for v1alpha1")
	assert.Equal(t, lint.SeverityWarning, iss.Severity)
}

func TestLint_NoSpecReturnsNil(t *testing.T) {
	t.Parallel()

	// Manifest with neither V1Alpha1 nor V1Alpha2 populated — Lint returns nil, not an empty slice.
	issues := lint.Lint(&oac.Manifest{})
	assert.Nil(t, issues)
}

func TestLint_EnvFile_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       oac.EnvFile
		expectError bool
	}{
		{"neither", oac.EnvFile{}, true},
		{"env only", oac.EnvFile{Env: "MY_TOKEN"}, false},
		{"file only", oac.EnvFile{File: "/run/secrets/token"}, false},
		{"both", oac.EnvFile{Env: "MY_TOKEN", File: "/run/secrets/token"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := &oac.Manifest{
				SpecVersion: oac.VersionV1Alpha2,
				V1Alpha2: &oac.V1Alpha2Spec{
					V1Alpha1Spec: oac.V1Alpha1Spec{
						Name: "test-agent",
						MCP: map[string]oac.MCPSpec{
							"srv": {
								Bearer: &oac.MCPBearerAuth{Token: tt.token},
							},
						},
					},
				},
			}

			issues := lint.Lint(m)
			iss := findIssue(issues, "mcp.srv.bearer.token")

			if tt.expectError {
				require.NotNil(t, iss, "expected error issue for %q", tt.name)
				assert.Equal(t, lint.SeverityError, iss.Severity)
			} else {
				assert.Nil(t, iss, "unexpected issue for %q: %v", tt.name, iss)
			}
		})
	}
}
