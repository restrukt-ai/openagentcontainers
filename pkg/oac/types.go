// Package oac provides parsing and validation for Open Agent Container (OAC) manifests.
//
// OAC encodes agent metadata as OCI image labels under the org.openagentcontainers.*
// namespace. [Parse] strips the prefix, converts dotted label suffixes into a JSON object
// tree, and decodes the result into a versioned [Manifest]. After a successful parse,
// exactly one of [Manifest.V1Alpha1] or [Manifest.V1Alpha2] is non-nil, selected by the
// version label. Call [Manifest.Validate] to check that required fields are populated.
//
//	m, err := oac.Parse(labels)
//	if err != nil { ... }
//	if err := m.Validate(); err != nil { ... }
//
// Use the discovery package to obtain label maps from OCI registries, and the lint
// package for advisory checks beyond what [Manifest.Validate] enforces.
package oac

import "fmt"

// Label key constants for OAC-conformant images.
const (
	LabelVersion = "org.openagentcontainers.version"
	LabelName    = "org.openagentcontainers.name"
	labelPrefix  = "org.openagentcontainers."
)

// LabelDescription is an unofficial extension recognized by [Parse] but absent from
// the OAC specification. Populated into [V1Alpha1Spec.Description] when present.
const LabelDescription = "org.openagentcontainers.description"

// VersionV1Alpha1 and VersionV1Alpha2 are the recognised values for [LabelVersion],
// selecting which versioned spec [Parse] decodes the remaining labels into.
const (
	VersionV1Alpha1 = "v1alpha1"
	VersionV1Alpha2 = "v1alpha2"
)

// Manifest is the parsed representation of an OAC image's labels.
// After a successful Parse call, exactly one of V1Alpha1 or V1Alpha2 will be
// non-nil, determined by the version label. Check Version or test each field for nil.
type Manifest struct {
	Version string `json:"version"`

	// V1Alpha1 is non-nil when Version is [VersionV1Alpha1]. Nil otherwise.
	V1Alpha1 *V1Alpha1Spec `json:"v1alpha1,omitempty"`

	// V1Alpha2 is non-nil when Version is [VersionV1Alpha2]. Nil otherwise.
	V1Alpha2 *V1Alpha2Spec `json:"v1alpha2,omitempty"`
}

// specValidator is implemented by each versioned spec.
type specValidator interface {
	validate() error
	orchestrator() *OrchestratorSpec
}

func (s *V1Alpha1Spec) orchestrator() *OrchestratorSpec { return s.Orchestrator }
func (s *V1Alpha2Spec) orchestrator() *OrchestratorSpec { return s.Orchestrator }

// validateSpec validates a versioned spec and its orchestrator.
func validateSpec(s specValidator) error {
	err := s.validate()
	if err != nil {
		return err
	}

	o := s.orchestrator()
	if o == nil {
		return ErrOrchestratorRequired
	}

	return validateOrchestrator(o)
}

// Validate checks that m contains a populated, correctly configured spec.
// Possible errors: [ErrNoSpec], [ErrNameRequired], [ErrOrchestratorRequired],
// [ErrOrchestratorEnvRequired], [ErrOrchestratorAuthRequired], and (v1alpha2 only)
// [ErrSessionIsolation]. Use [errors.Is] to test for specific conditions.
func (m *Manifest) Validate() error {
	switch {
	case m.V1Alpha1 != nil:
		return validateSpec(m.V1Alpha1)
	case m.V1Alpha2 != nil:
		return validateSpec(m.V1Alpha2)
	default:
		return fmt.Errorf("%w %q", ErrNoSpec, m.Version)
	}
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
// Name and Orchestrator are required (enforced by Validate). All other fields are optional.
// Description is populated from LabelDescription, which is an unofficial extension not defined
// by the OAC specification.
type V1Alpha1Spec struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Inference   *InferenceSpec           `json:"inference,omitempty"`
	MCP         map[string]MCPSpec       `json:"mcp,omitempty"`
	Workspace   map[string]WorkspaceSpec `json:"workspace,omitempty"`
	// Orchestrator is required; [Validate] returns [ErrOrchestratorRequired] when nil.
	Orchestrator *OrchestratorSpec    `json:"orchestrator,omitempty"`
	Events       map[string]EventSpec `json:"events,omitempty"`
}

func (s *V1Alpha1Spec) validate() error {
	if s.Name == "" {
		return ErrNameRequired
	}

	return nil
}

// V1Alpha2Spec is the spec for OAC images declaring version "v1alpha2".
// It extends V1Alpha1Spec with session isolation support.
// When Session.Isolation is true, the Workspace map must be empty; combining them causes
// Validate to return ErrSessionIsolation.
type V1Alpha2Spec struct {
	V1Alpha1Spec

	// Session configures per-session isolation. When Session.Isolation is true,
	// Workspace must be empty; see [ErrSessionIsolation].
	Session SessionSpec `json:"session"`
}

func (s *V1Alpha2Spec) validate() error {
	err := s.V1Alpha1Spec.validate()
	if err != nil {
		return err
	}

	if s.Session.Isolation && len(s.Workspace) > 0 {
		return ErrSessionIsolation
	}

	return nil
}

// SessionSpec describes per-session runtime isolation settings.
type SessionSpec struct {
	// Isolation, when true, requests that the orchestrator provision a fresh ephemeral
	// workspace per session rather than sharing volumes across sessions.
	// Incompatible with declared workspaces; see ErrSessionIsolation.
	Isolation bool `json:"isolation,omitempty"`
}

// EnvFile describes where a credential value is delivered at runtime.
// Env is the name of an environment variable; File is an absolute filesystem path.
// The orchestrator injects the credential value into whichever source(s) are declared.
// Setting both allows fallback: the runtime checks Env first, then File.
// At least one field must be set.
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
	// Models is a space-separated list of model identifiers accepted by this inference
	// type, e.g. "gpt-4o llama-3.1-8b-instruct".
	Models string `json:"models,omitempty"`
}

// MCPSpec describes an MCP server's authentication configuration.
// Set exactly one of Bearer, OAuth, or DCR per server entry. The OAC specification defines
// these as mutually exclusive auth strategies; behaviour when multiple are set is
// orchestrator-defined and may vary.
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
	// Scopes is an optional space-separated list of OAuth scopes to request
	// during Dynamic Client Registration.
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
	// Env is the name of the environment variable the orchestrator injects at container
	// start with the orchestrator endpoint URL.
	Env    string                  `json:"env,omitempty"`
	Bearer *OrchestratorBearerAuth `json:"bearer,omitempty"`
	MTLS   *OrchestratorMTLSAuth   `json:"mtls,omitempty"`
}

// OrchestratorBearerAuth carries a bearer token for orchestrator authentication.
type OrchestratorBearerAuth struct {
	Token EnvFile `json:"token"`
}

// OrchestratorMTLSAuth carries mTLS credentials for orchestrator authentication.
// Cert and Key are required credential sources. CA is optional — the orchestrator provisions
// the CA certificate; Lint does not warn when CA is absent.
type OrchestratorMTLSAuth struct {
	Cert EnvFile `json:"cert"`
	Key  EnvFile `json:"key"`
	CA   EnvFile `json:"ca"`
}

// EventSpec describes an event subscription with an embedded schema.
type EventSpec struct {
	// Schema describes the location and format of the event payload schema
	// embedded in the container image.
	Schema EventSchema `json:"schema"`
}

// EventSchema holds the path and MIME type of the event schema file.
// Path is an absolute path inside the container image to the schema file.
// MIMEType is the MIME type of the schema file, e.g. "application/json" or "application/yaml".
type EventSchema struct {
	Path     string `json:"path,omitempty"`
	MIMEType string `json:"mimetype,omitempty"`
}
