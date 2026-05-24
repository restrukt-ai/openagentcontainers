// Package oac provides types, parsing, and validation for Open Agent Container manifests.
package oac

import "fmt"

// Label key constants for OAC-conformant images.
const (
	LabelVersion     = "org.openagentcontainers.version"
	LabelName        = "org.openagentcontainers.name"
	LabelDescription = "org.openagentcontainers.description"
	labelPrefix      = "org.openagentcontainers."

	VersionV1Alpha1 = "v1alpha1"
	VersionV1Alpha2 = "v1alpha2"
)

// Manifest is the parsed representation of an OAC image's labels.
type Manifest struct {
	Version  string        `json:"version"`
	V1Alpha1 *V1Alpha1Spec `json:"v1alpha1,omitempty"`
	V1Alpha2 *V1Alpha2Spec `json:"v1alpha2,omitempty"`
}

// Validate checks that m contains a valid, populated spec with an orchestrator configured.
func (m *Manifest) Validate() error {
	switch {
	case m.V1Alpha1 != nil:
		err := m.V1Alpha1.validate()
		if err != nil {
			return err
		}

		if m.V1Alpha1.Orchestrator == nil {
			return ErrOrchestratorRequired
		}
	case m.V1Alpha2 != nil:
		err := m.V1Alpha2.validate()
		if err != nil {
			return err
		}

		if m.V1Alpha2.Orchestrator == nil {
			return ErrOrchestratorRequired
		}
	default:
		return fmt.Errorf("%w %q", ErrNoSpec, m.Version)
	}

	return nil
}

// V1Alpha1Spec is the spec for OAC images declaring version "v1alpha1".
type V1Alpha1Spec struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	Inference    InferenceSpec            `json:"inference"`
	MCP          map[string]MCPSpec       `json:"mcp,omitempty"`
	Workspaces   map[string]WorkspaceSpec `json:"workspaces,omitempty"`
	Orchestrator *OrchestratorSpec        `json:"orchestrator,omitempty"`
	Events       map[string]EventSpec     `json:"events,omitempty"`
}

func (s *V1Alpha1Spec) validate() error {
	if s.Name == "" {
		return ErrNameRequired
	}

	return nil
}

// V1Alpha2Spec is the spec for OAC images declaring version "v1alpha2".
// It extends V1Alpha1Spec with session isolation support.
type V1Alpha2Spec struct {
	V1Alpha1Spec

	Session SessionSpec `json:"session"`
}

func (s *V1Alpha2Spec) validate() error {
	err := s.V1Alpha1Spec.validate()
	if err != nil {
		return err
	}

	if s.Session.Isolation && len(s.Workspaces) > 0 {
		return ErrSessionIsolation
	}

	return nil
}

// SessionSpec describes per-session runtime isolation settings.
type SessionSpec struct {
	Isolation bool `json:"isolation,omitempty"`
}

// InferenceSpec describes the agent's inference configuration.
type InferenceSpec struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// InferenceTypeSpec represents a supported inference backend type.
type InferenceTypeSpec struct {
	Name string `json:"name"`
}

// MCPSpec describes an MCP server endpoint and its authentication.
type MCPSpec struct {
	URL  string   `json:"url,omitempty"`
	Auth *MCPAuth `json:"auth,omitempty"`
}

// MCPAuth holds authentication configuration for an MCP server.
type MCPAuth struct {
	Bearer *MCPBearerAuth `json:"bearer,omitempty"`
	OAuth  *MCPOAuthAuth  `json:"oauth,omitempty"`
	DCR    *MCPDCRAuth    `json:"dcr,omitempty"`
}

// MCPBearerAuth carries a bearer token credential for an MCP server.
type MCPBearerAuth struct {
	Token CredentialTarget `json:"token"`
}

// MCPOAuthAuth carries OAuth2 client credentials for an MCP server.
type MCPOAuthAuth struct {
	ClientID     string           `json:"client-id,omitempty"`
	ClientSecret CredentialTarget `json:"client-secret"`
}

// MCPDCRAuth carries Dynamic Client Registration credentials for an MCP server.
type MCPDCRAuth struct {
	Endpoint   string           `json:"endpoint,omitempty"`
	Credential CredentialTarget `json:"credential"`
}

// CredentialTarget describes where to read a secret value at runtime.
type CredentialTarget struct {
	EnvVar string `json:"env,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// WorkspaceSpec describes a mounted workspace volume.
type WorkspaceSpec struct {
	Type string `json:"type,omitempty"`
	Path string `json:"path,omitempty"`
}

// OrchestratorSpec describes the orchestrator endpoint and its authentication.
type OrchestratorSpec struct {
	URL  string            `json:"url,omitempty"`
	Auth *OrchestratorAuth `json:"auth,omitempty"`
}

// OrchestratorAuth holds authentication configuration for an orchestrator.
type OrchestratorAuth struct {
	Bearer *OrchestratorBearerAuth `json:"bearer,omitempty"`
	MTLS   *OrchestratorMTLSAuth   `json:"mtls,omitempty"`
}

// OrchestratorBearerAuth carries a bearer token for orchestrator authentication.
type OrchestratorBearerAuth struct {
	Token CredentialTarget `json:"token"`
}

// OrchestratorMTLSAuth carries mTLS credentials for orchestrator authentication.
type OrchestratorMTLSAuth struct {
	Cert CredentialTarget `json:"cert"`
	Key  CredentialTarget `json:"key"`
}

// EventSpec describes an event trigger.
type EventSpec struct {
	Type    string `json:"type,omitempty"`
	Channel string `json:"channel,omitempty"`
}
