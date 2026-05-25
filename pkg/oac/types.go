// Package oac provides types, parsing, and validation for Open Agent Container manifests.
package oac

import "fmt"

// Label key constants for OAC-conformant images.
const (
	LabelVersion     = "org.openagentcontainers.version"
	LabelName        = "org.openagentcontainers.name"
	LabelDescription = "org.openagentcontainers.description" // unofficial extension; not in spec
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

// Validate checks that m contains a valid, populated spec with a correctly configured orchestrator.
func (m *Manifest) Validate() error {
	switch {
	case m.V1Alpha1 != nil:
		if err := m.V1Alpha1.validate(); err != nil {
			return err
		}

		if m.V1Alpha1.Orchestrator == nil {
			return ErrOrchestratorRequired
		}

		if err := validateOrchestrator(m.V1Alpha1.Orchestrator); err != nil {
			return err
		}
	case m.V1Alpha2 != nil:
		if err := m.V1Alpha2.validate(); err != nil {
			return err
		}

		if m.V1Alpha2.Orchestrator == nil {
			return ErrOrchestratorRequired
		}

		if err := validateOrchestrator(m.V1Alpha2.Orchestrator); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w %q", ErrNoSpec, m.Version)
	}

	return nil
}

func validateOrchestrator(o *OrchestratorSpec) error {
	if o.Env == "" {
		return ErrOrchestratorEnvRequired
	}

	if o.Bearer == nil && o.MTLS == nil {
		return ErrOrchestratorAuthRequired
	}

	return nil
}

// V1Alpha1Spec is the spec for OAC images declaring version "v1alpha1".
type V1Alpha1Spec struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"` // unofficial extension; not in spec
	Inference    *InferenceSpec           `json:"inference,omitempty"`
	MCP          map[string]MCPSpec       `json:"mcp,omitempty"`
	Workspace    map[string]WorkspaceSpec `json:"workspace,omitempty"`
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
	if err := s.V1Alpha1Spec.validate(); err != nil {
		return err
	}

	if s.Session.Isolation && len(s.Workspace) > 0 {
		return ErrSessionIsolation
	}

	return nil
}

// SessionSpec describes per-session runtime isolation settings.
type SessionSpec struct {
	Isolation bool `json:"isolation,omitempty"`
}

// EnvFile describes where a credential value is delivered at runtime.
// At least one field must be set. Both may be set simultaneously.
type EnvFile struct {
	Env  string `json:"env,omitempty"`
	File string `json:"file,omitempty"`
}

// InferenceSpec describes the agent's inference configuration.
// Types is populated by a custom UnmarshalJSON and holds per-type model lists.
type InferenceSpec struct {
	APIBase *EnvFile                     `json:"api_base,omitempty"`
	APIKey  *EnvFile                     `json:"api_key,omitempty"`
	Types   map[string]InferenceTypeSpec `json:"-"` // populated by UnmarshalJSON
}

// InferenceTypeSpec holds the model list for a single inference type.
type InferenceTypeSpec struct {
	Models string `json:"models,omitempty"`
}

// MCPSpec describes an MCP server's authentication configuration.
type MCPSpec struct {
	Bearer *MCPBearerAuth `json:"bearer,omitempty"`
	OAuth  *MCPOAuthAuth  `json:"oauth,omitempty"`
	DCR    *MCPDCRAuth    `json:"dcr,omitempty"`
}

// MCPBearerAuth carries a bearer token credential for an MCP server.
type MCPBearerAuth struct {
	Token EnvFile `json:"token"`
}

// MCPOAuthAuth carries OAuth2 client credentials for an MCP server.
type MCPOAuthAuth struct {
	ClientID     EnvFile `json:"client_id"`
	ClientSecret EnvFile `json:"client_secret"`
}

// MCPDCRAuth carries Dynamic Client Registration credentials for an MCP server.
type MCPDCRAuth struct {
	Scopes       string  `json:"scopes,omitempty"`
	ClientID     EnvFile `json:"client_id"`
	ClientSecret EnvFile `json:"client_secret"`
}

// WorkspaceSpec describes a mounted workspace volume.
type WorkspaceSpec struct {
	Path    string `json:"path,omitempty"`
	Mutable bool   `json:"mutable,omitempty"`
}

// OrchestratorSpec describes the orchestrator endpoint and its authentication.
type OrchestratorSpec struct {
	Env    string                  `json:"env,omitempty"`
	Bearer *OrchestratorBearerAuth `json:"bearer,omitempty"`
	MTLS   *OrchestratorMTLSAuth   `json:"mtls,omitempty"`
}

// OrchestratorBearerAuth carries a bearer token for orchestrator authentication.
type OrchestratorBearerAuth struct {
	Token EnvFile `json:"token"`
}

// OrchestratorMTLSAuth carries mTLS credentials for orchestrator authentication.
type OrchestratorMTLSAuth struct {
	Cert EnvFile `json:"cert"`
	Key  EnvFile `json:"key"`
	CA   EnvFile `json:"ca"`
}

// EventSpec describes an event subscription with an embedded schema.
type EventSpec struct {
	Schema EventSchema `json:"schema"`
}

// EventSchema holds the path and MIME type of the event schema file.
type EventSchema struct {
	Path     string `json:"path,omitempty"`
	MIMEType string `json:"mimetype,omitempty"`
}
