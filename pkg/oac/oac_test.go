package oac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func TestParse_V1Alpha2_BasicFields(t *testing.T) {
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
		"org.openagentcontainers.version":              "v1alpha2",
		"org.openagentcontainers.name":                 "full-agent",
		"org.openagentcontainers.description":          "A fully-labelled agent",
		"org.openagentcontainers.inference.provider":   "openai",
		"org.openagentcontainers.inference.model":      "gpt-4",
		"org.openagentcontainers.mcp.my-server.url":    "http://mcp.example.com",
		"org.openagentcontainers.workspaces.code.path": "/workspace/code",
		"org.openagentcontainers.workspaces.code.type": "git",
		"org.openagentcontainers.orchestrator.url":     "http://orchestrator.example.com",
		"org.openagentcontainers.events.start.type":    "webhook",
		"org.openagentcontainers.events.start.channel": "start-channel",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)

	s := m.V1Alpha2
	assert.Equal(t, "full-agent", s.Name)
	assert.Equal(t, "A fully-labelled agent", s.Description)
	assert.Equal(t, "openai", s.Inference.Provider)
	assert.Equal(t, "gpt-4", s.Inference.Model)
	require.Contains(t, s.MCP, "my-server")
	assert.Equal(t, "http://mcp.example.com", s.MCP["my-server"].URL)
	require.Contains(t, s.Workspaces, "code")
	assert.Equal(t, "/workspace/code", s.Workspaces["code"].Path)
	assert.Equal(t, "git", s.Workspaces["code"].Type)
	require.NotNil(t, s.Orchestrator)
	assert.Equal(t, "http://orchestrator.example.com", s.Orchestrator.URL)
	require.Contains(t, s.Events, "start")
	assert.Equal(t, "webhook", s.Events["start"].Type)
	assert.Equal(t, "start-channel", s.Events["start"].Channel)
}

func TestParse_V1Alpha1_BasicFields(t *testing.T) {
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

func TestParse_SessionIsolationTrue(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":           "v1alpha2",
		"org.openagentcontainers.name":              "isolated-agent",
		"org.openagentcontainers.session.isolation": "true",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	assert.True(t, m.V1Alpha2.Session.Isolation)
}

func TestParse_SessionIsolationDefault(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version": "v1alpha2",
		"org.openagentcontainers.name":    "agent",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	assert.False(t, m.V1Alpha2.Session.Isolation)
}

func TestParse_UnknownVersion_Error(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version": "v99beta1",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v99beta1")
}

func TestParse_UnknownLabel_Error(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":     "v1alpha2",
		"org.openagentcontainers.name":        "agent",
		"org.openagentcontainers.unknown-key": "value",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
}

func TestParse_V1Alpha1_SessionIsolation_Error(t *testing.T) {
	t.Parallel()

	// session.isolation is not a valid v1alpha1 field
	labels := map[string]string{
		"org.openagentcontainers.version":           "v1alpha1",
		"org.openagentcontainers.name":              "agent",
		"org.openagentcontainers.session.isolation": "true",
	}

	_, err := oac.Parse(labels)
	require.Error(t, err)
}

func TestValidate_ValidMinimal(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name:         "my-agent",
				Orchestrator: &oac.OrchestratorSpec{URL: "http://orch.example.com"},
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

	assert.Error(t, m.Validate())
}

func TestValidate_NoSpecPopulated(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{Version: "v99beta1"}
	assert.Error(t, m.Validate())
}

func TestValidate_SessionIsolationWithWorkspace(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name: "agent",
				Workspaces: map[string]oac.WorkspaceSpec{
					"code": {Path: "/workspace"},
				},
			},
			Session: oac.SessionSpec{Isolation: true},
		},
	}

	assert.Error(t, m.Validate())
}

func TestValidate_SessionIsolationNoWorkspace(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name:         "agent",
				Orchestrator: &oac.OrchestratorSpec{URL: "http://orch.example.com"},
			},
			Session: oac.SessionSpec{Isolation: true},
		},
	}

	assert.NoError(t, m.Validate())
}

func TestValidate_RequiresOrchestrator(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{Name: "agent"},
		},
	}

	assert.Error(t, m.Validate())
}

func TestValidate_WithOrchestrator(t *testing.T) {
	t.Parallel()

	m := &oac.Manifest{
		Version: "v1alpha2",
		V1Alpha2: &oac.V1Alpha2Spec{
			V1Alpha1Spec: oac.V1Alpha1Spec{
				Name:         "agent",
				Orchestrator: &oac.OrchestratorSpec{URL: "http://orch.example.com"},
			},
		},
	}

	assert.NoError(t, m.Validate())
}

func TestParse_MCPBearerAuth(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                             "v1alpha2",
		"org.openagentcontainers.name":                                "agent",
		"org.openagentcontainers.mcp.my-server.url":                   "http://mcp.example.com",
		"org.openagentcontainers.mcp.my-server.auth.bearer.token.env": "MY_TOKEN",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.Contains(t, m.V1Alpha2.MCP, "my-server")
	require.NotNil(t, m.V1Alpha2.MCP["my-server"].Auth)
	require.NotNil(t, m.V1Alpha2.MCP["my-server"].Auth.Bearer)
	assert.Equal(t, "MY_TOKEN", m.V1Alpha2.MCP["my-server"].Auth.Bearer.Token.EnvVar)
}

func TestParse_OrchestratorBearerAuth(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                            "v1alpha2",
		"org.openagentcontainers.name":                               "agent",
		"org.openagentcontainers.orchestrator.url":                   "http://orch.example.com",
		"org.openagentcontainers.orchestrator.auth.bearer.token.env": "ORCH_TOKEN",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.NotNil(t, m.V1Alpha2.Orchestrator)
	require.NotNil(t, m.V1Alpha2.Orchestrator.Auth)
	require.NotNil(t, m.V1Alpha2.Orchestrator.Auth.Bearer)
	assert.Equal(t, "ORCH_TOKEN", m.V1Alpha2.Orchestrator.Auth.Bearer.Token.EnvVar)
}

func TestParse_OrchestratorMTLS(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"org.openagentcontainers.version":                         "v1alpha2",
		"org.openagentcontainers.name":                            "agent",
		"org.openagentcontainers.orchestrator.url":                "http://orch.example.com",
		"org.openagentcontainers.orchestrator.auth.mtls.cert.env": "ORCH_CERT",
		"org.openagentcontainers.orchestrator.auth.mtls.key.env":  "ORCH_KEY",
	}

	m, err := oac.Parse(labels)
	require.NoError(t, err)
	require.NotNil(t, m.V1Alpha2)
	require.NotNil(t, m.V1Alpha2.Orchestrator)
	require.NotNil(t, m.V1Alpha2.Orchestrator.Auth)
	require.NotNil(t, m.V1Alpha2.Orchestrator.Auth.MTLS)
	assert.Equal(t, "ORCH_CERT", m.V1Alpha2.Orchestrator.Auth.MTLS.Cert.EnvVar)
	assert.Equal(t, "ORCH_KEY", m.V1Alpha2.Orchestrator.Auth.MTLS.Key.EnvVar)
}
