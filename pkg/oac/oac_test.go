package oac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// --- Parsing tests ---

func TestParse_V1Alpha2_MinimalValid(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version": "v1alpha2",
		"org.openagentcontainers.name":    "my-agent",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	assert.Equal(t, "v1alpha2", m.Version)
	assert.Equal(t, "my-agent", m.V1Alpha2.Name)
}

func TestParse_V1Alpha2_FullLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                           "v1alpha2",
		"org.openagentcontainers.name":                              "full-agent",
		"org.openagentcontainers.description":                       "A fully-labelled agent",
		"org.openagentcontainers.orchestrator.env":                  "ORCHESTRATOR_ADDR",
		"org.openagentcontainers.orchestrator.bearer.token.env":     "ORCH_TOKEN",
		"org.openagentcontainers.inference.api_base.env":            "OPENAI_BASE_URL",
		"org.openagentcontainers.inference.api_key.env":             "OPENAI_API_KEY",
		"org.openagentcontainers.inference.chat-completions.models": "gpt-4o",
		"org.openagentcontainers.inference.embeddings.models":       "text-embedding-004",
		"org.openagentcontainers.mcp.srv.bearer.token.env":          "SRV_TOKEN",
		"org.openagentcontainers.workspace.code.path":               "/workspace/code",
		"org.openagentcontainers.workspace.code.mutable":            "true",
		"org.openagentcontainers.events.start.schema.path":          "/oac/schemas/start.json",
		"org.openagentcontainers.events.start.schema.mimetype":      "application/schema+json",
		"org.openagentcontainers.session.isolation":                 "false",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)

	s := m.V1Alpha2
	assert.Equal(t, "full-agent", s.Name)
	assert.Equal(t, "A fully-labelled agent", s.Description)

	require.NotNil(t, s.Orchestrator)
	assert.Equal(t, "ORCHESTRATOR_ADDR", s.Orchestrator.Env)
	require.NotNil(t, s.Orchestrator.Bearer)
	assert.Equal(t, "ORCH_TOKEN", s.Orchestrator.Bearer.Token.Env)

	require.NotNil(t, s.Inference)
	require.NotNil(t, s.Inference.APIBase)
	assert.Equal(t, "OPENAI_BASE_URL", s.Inference.APIBase.Env)
	require.NotNil(t, s.Inference.APIKey)
	assert.Equal(t, "OPENAI_API_KEY", s.Inference.APIKey.Env)
	require.Contains(t, s.Inference.Types, "chat-completions")
	assert.Equal(t, "gpt-4o", s.Inference.Types["chat-completions"].Models)
	require.Contains(t, s.Inference.Types, "embeddings")
	assert.Equal(t, "text-embedding-004", s.Inference.Types["embeddings"].Models)

	require.Contains(t, s.MCP, "srv")
	require.NotNil(t, s.MCP["srv"].Bearer)
	assert.Equal(t, "SRV_TOKEN", s.MCP["srv"].Bearer.Token.Env)

	require.Contains(t, s.Workspace, "code")
	assert.Equal(t, "/workspace/code", s.Workspace["code"].Path)
	assert.True(t, s.Workspace["code"].Mutable)

	require.Contains(t, s.Events, "start")
	assert.Equal(t, "/oac/schemas/start.json", s.Events["start"].Schema.Path)
	assert.Equal(t, "application/schema+json", s.Events["start"].Schema.MIMEType)

	assert.False(t, s.Session.Isolation)
}

func TestParse_InferenceMultipleTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeKey string
		models  string
	}{
		{"chat-completions", "chat-completions", "gpt-4o"},
		{"embeddings", "embeddings", "text-embedding-004"},
		{"audio-transcriptions", "audio-transcriptions", "whisper-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			labels := map[string]string{
				"org.openagentcontainers.version":                             "v1alpha2",
				"org.openagentcontainers.name":                                "agent",
				"org.openagentcontainers.inference." + tt.typeKey + ".models": tt.models,
			}

			m, err := oac.Parse(labels)
			require.NoError(t, err)
			require.NotNil(t, m.V1Alpha2.Inference)
			require.Contains(t, m.V1Alpha2.Inference.Types, tt.typeKey)
			assert.Equal(t, tt.models, m.V1Alpha2.Inference.Types[tt.typeKey].Models)
		})
	}
}

func TestParse_InferenceUnknownSubField(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                            "v1alpha2",
		"org.openagentcontainers.name":                               "agent",
		"org.openagentcontainers.inference.chat-completions.unknown": "x",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
}

func TestParse_MCPBearer(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                   "v1alpha2",
		"org.openagentcontainers.name":                      "agent",
		"org.openagentcontainers.mcp.srv.bearer.token.env":  "MY_TOKEN",
		"org.openagentcontainers.mcp.srv.bearer.token.file": "/run/secrets/token",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.Contains(t, m.V1Alpha2.MCP, "srv")
	srv := m.V1Alpha2.MCP["srv"]
	require.NotNil(t, srv.Bearer)
	assert.Equal(t, "MY_TOKEN", srv.Bearer.Token.Env)
	assert.Equal(t, "/run/secrets/token", srv.Bearer.Token.File)
}

func TestParse_MCPOAuth(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                         "v1alpha2",
		"org.openagentcontainers.name":                            "agent",
		"org.openagentcontainers.mcp.srv.oauth.client_id.env":     "MY_CLIENT_ID",
		"org.openagentcontainers.mcp.srv.oauth.client_secret.env": "MY_CLIENT_SECRET",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.Contains(t, m.V1Alpha2.MCP, "srv")
	srv := m.V1Alpha2.MCP["srv"]
	require.NotNil(t, srv.OAuth)
	assert.Equal(t, "MY_CLIENT_ID", srv.OAuth.ClientID.Env)
	assert.Equal(t, "MY_CLIENT_SECRET", srv.OAuth.ClientSecret.Env)
}

func TestParse_MCPDCRWithScopes(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                        "v1alpha2",
		"org.openagentcontainers.name":                           "agent",
		"org.openagentcontainers.mcp.srv.dcr.scopes":             "repo:read repo:write",
		"org.openagentcontainers.mcp.srv.dcr.client_id.env":      "MY_CLIENT_ID",
		"org.openagentcontainers.mcp.srv.dcr.client_secret.file": "/run/secrets/secret",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.Contains(t, m.V1Alpha2.MCP, "srv")
	srv := m.V1Alpha2.MCP["srv"]
	require.NotNil(t, srv.DCR)
	assert.Equal(t, "repo:read repo:write", srv.DCR.Scopes)
	assert.Equal(t, "MY_CLIENT_ID", srv.DCR.ClientID.Env)
	assert.Equal(t, "/run/secrets/secret", srv.DCR.ClientSecret.File)
}

func TestParse_OrchestratorMTLS(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                     "v1alpha2",
		"org.openagentcontainers.name":                        "agent",
		"org.openagentcontainers.orchestrator.env":            "ORCHESTRATOR_ADDR",
		"org.openagentcontainers.orchestrator.mtls.cert.file": "/run/secrets/harness.crt",
		"org.openagentcontainers.orchestrator.mtls.key.file":  "/run/secrets/harness.key",
		"org.openagentcontainers.orchestrator.mtls.ca.file":   "/run/secrets/ca.crt",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.NotNil(t, m.V1Alpha2.Orchestrator)
	orch := m.V1Alpha2.Orchestrator
	assert.Equal(t, "ORCHESTRATOR_ADDR", orch.Env)
	require.NotNil(t, orch.MTLS)
	assert.Equal(t, "/run/secrets/harness.crt", orch.MTLS.Cert.File)
	assert.Equal(t, "/run/secrets/harness.key", orch.MTLS.Key.File)
	assert.Equal(t, "/run/secrets/ca.crt", orch.MTLS.CA.File)
}

func TestParse_WorkspaceMutable(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                "v1alpha2",
		"org.openagentcontainers.name":                   "agent",
		"org.openagentcontainers.workspace.repo.path":    "/workspace/repo",
		"org.openagentcontainers.workspace.repo.mutable": "true",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.Contains(t, m.V1Alpha2.Workspace, "repo")
	ws := m.V1Alpha2.Workspace["repo"]
	assert.Equal(t, "/workspace/repo", ws.Path)
	assert.True(t, ws.Mutable)
}

func TestParse_V1Alpha1_Basic(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":     "v1alpha1",
		"org.openagentcontainers.name":        "v1agent",
		"org.openagentcontainers.description": "A v1alpha1 agent",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha1)
	assert.Nil(t, m.V1Alpha2)
	assert.Equal(t, "v1agent", m.V1Alpha1.Name)
	assert.Equal(t, "A v1alpha1 agent", m.V1Alpha1.Description)
}

func TestParse_UnknownVersion(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version": "v99beta1",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
	require.ErrorIs(t, err, oac.ErrUnsupportedVersion)
	assert.Contains(t, err.Error(), "v99beta1")
}

func TestParse_UnknownLabel(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":     "v1alpha2",
		"org.openagentcontainers.name":        "agent",
		"org.openagentcontainers.unknown-key": "value",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
}

func TestParse_SessionIsolationTrue(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":           "v1alpha2",
		"org.openagentcontainers.name":              "agent",
		"org.openagentcontainers.session.isolation": "true",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	assert.True(t, m.V1Alpha2.Session.Isolation)
}

func TestParse_V1Alpha1_SessionIsolation_Error(t *testing.T) {
	t.Parallel()

	// session is not a valid v1alpha1 field
	labels := map[string]string{
		"org.openagentcontainers.version":           "v1alpha1",
		"org.openagentcontainers.name":              "agent",
		"org.openagentcontainers.session.isolation": "true",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
}

// --- Validation tests ---

func TestValidate_Valid(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "my-agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
					Bearer: &oac.OrchestratorBearerAuth{
						Token: oac.EnvFile{Env: "ORCH_TOKEN"},
					},
				},
			},
		},
	}

	assert.NoError(t, m.Validate())
}

func TestValidate_MissingName(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version:  "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrNameRequired)
}

func TestValidate_MissingOrchestrator(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{Name: "agent"},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorRequired)
}

func TestValidate_OrchestratorEnvEmpty(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "agent",
				Orchestrator: &oac.OrchestratorSpec{
					Bearer: &oac.OrchestratorBearerAuth{
						Token: oac.EnvFile{Env: "ORCH_TOKEN"},
					},
				},
			},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorEnvRequired)
}

func TestValidate_OrchestratorNoAuth(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
				},
			},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorAuthRequired)
}

func TestValidate_SessionIsolationWithWorkspace(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "agent",
				Workspace: map[string]oac.WorkspaceSpec{
					"code": {Path: "/workspace"},
				},
			},
			Session: oac.SessionSpec{Isolation: true},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrSessionIsolation)
}

func TestValidate_SessionIsolationNoWorkspace(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "agent",
				Orchestrator: &oac.OrchestratorSpec{
					Env: "ORCHESTRATOR_ADDR",
					Bearer: &oac.OrchestratorBearerAuth{
						Token: oac.EnvFile{Env: "ORCH_TOKEN"},
					},
				},
			},
			Session: oac.SessionSpec{Isolation: true},
		},
	}

	assert.NoError(t, m.Validate())
}

func TestValidate_NoSpec(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{Version: "v99beta1"}
	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrNoSpec)
}

// --- V1Alpha1 Validate paths ---

func TestValidate_V1Alpha1_RequiresOrchestrator(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version:  "v1alpha1",
		V1Alpha1: &oac.V1Alpha1Spec{Name: "agent"},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorRequired)
}

func TestValidate_V1Alpha1_OrchestratorEnvEmpty(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha1",
		V1Alpha1: &oac.V1Alpha1Spec{
			Name: "agent",
			Orchestrator: &oac.OrchestratorSpec{
				Bearer: &oac.OrchestratorBearerAuth{
					Token: oac.EnvFile{Env: "ORCH_TOKEN"},
				},
			},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorEnvRequired)
}

func TestValidate_V1Alpha1_OrchestratorNoAuth(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha1",
		V1Alpha1: &oac.V1Alpha1Spec{
			Name: "agent",
			Orchestrator: &oac.OrchestratorSpec{
				Env: "ORCHESTRATOR_ADDR",
			},
		},
	}

	err := m.Validate()
	require.Error(t, err)
	assert.ErrorIs(t, err, oac.ErrOrchestratorAuthRequired)
}

// --- labelsToTree non-OAC prefix skip ---

func TestParse_NonOACLabelIgnored(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"com.docker.foo":                  "bar",
		"org.openagentcontainers.version": "v1alpha2",
		"org.openagentcontainers.name":    "agent",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	assert.Equal(t, "agent", m.V1Alpha2.Name)
}

// --- InferenceSpec.UnmarshalJSON scalar error paths ---

func TestParse_InferenceAPIBaseAsScalar(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":            "v1alpha2",
		"org.openagentcontainers.name":               "agent",
		"org.openagentcontainers.inference.api_base": "OPENAI_BASE_URL",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inference.api_base")
}

func TestParse_InferenceAPIKeyAsScalar(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":           "v1alpha2",
		"org.openagentcontainers.name":              "agent",
		"org.openagentcontainers.inference.api_key": "OPENAI_API_KEY",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inference.api_key")
}
