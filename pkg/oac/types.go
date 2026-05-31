// Package oac provides parsing for Open Agent Container (OAC) manifests.
//
// OAC encodes agent metadata as OCI image labels under the org.openagentcontainers.*
// namespace. [Parse] strips the prefix, converts dotted label suffixes into a JSON object
// tree, and decodes the result into a versioned [Manifest]. After a successful parse,
// exactly one of [Manifest.V1Alpha1], [Manifest.V1Alpha2], or [Manifest.V1Alpha3] is
// non-nil, selected by the version label.
//
//	m, err := oac.Parse(labels)
//	if err != nil { ... }
//
// Use the discovery package to obtain label maps from OCI registries, and the check
// package for all validation and advisory checks.
package oac

// Label key constants for OAC-conformant images.
const (
	LabelVersion = "org.openagentcontainers.version"
	LabelName    = "org.openagentcontainers.name"
	LabelPrefix  = "org.openagentcontainers."
)

// LabelDescription is an unofficial extension recognized by [Parse] but absent from
// the OAC specification. Populated into the active spec's Description field when present.
const LabelDescription = "org.openagentcontainers.description"

// SpecVersion is the type for OAC spec version identifiers.
type SpecVersion string

// VersionV1Alpha1, VersionV1Alpha2, and VersionV1Alpha3 are the recognised [SpecVersion]
// values for [LabelVersion], selecting which versioned spec [Parse] decodes the remaining
// labels into.
const (
	VersionV1Alpha1 SpecVersion = "v1alpha1"
	VersionV1Alpha2 SpecVersion = "v1alpha2"
	VersionV1Alpha3 SpecVersion = "v1alpha3"
)

// Manifest is the parsed representation of an OAC image's labels.
// After a successful Parse call, exactly one of V1Alpha1, V1Alpha2, or V1Alpha3 will be
// non-nil, determined by the version label. Check SpecVersion or test each field for nil.
// Call [Manifest.Name] and [Manifest.Description] to read those fields without switching
// on SpecVersion yourself.
type Manifest struct {
	SpecVersion SpecVersion `json:"spec_version"`

	// V1Alpha1 is non-nil when SpecVersion is [VersionV1Alpha1]. Nil otherwise.
	V1Alpha1 *V1Alpha1Spec `json:"v1alpha1,omitempty"`

	// V1Alpha2 is non-nil when SpecVersion is [VersionV1Alpha2]. Nil otherwise.
	V1Alpha2 *V1Alpha2Spec `json:"v1alpha2,omitempty"`

	// V1Alpha3 is non-nil when SpecVersion is [VersionV1Alpha3]. Nil otherwise.
	V1Alpha3 *V1Alpha3Spec `json:"v1alpha3,omitempty"`
}

// Name returns the agent name from whichever versioned spec is populated, or "" if none.
func (m *Manifest) Name() string {
	switch {
	case m.V1Alpha1 != nil:
		return m.V1Alpha1.Name
	case m.V1Alpha2 != nil:
		return m.V1Alpha2.Name
	case m.V1Alpha3 != nil:
		return m.V1Alpha3.Name
	default:
		return ""
	}
}

// Description returns the agent description from whichever versioned spec is populated, or "" if none.
func (m *Manifest) Description() string {
	switch {
	case m.V1Alpha1 != nil:
		return m.V1Alpha1.Description
	case m.V1Alpha2 != nil:
		return m.V1Alpha2.Description
	case m.V1Alpha3 != nil:
		return m.V1Alpha3.Description
	default:
		return ""
	}
}

// V1Alpha1Spec is the spec for OAC images declaring [VersionV1Alpha1].
// All fields are optional at the parse level; use [check.Check] to validate required fields.
// Description is populated from [LabelDescription], which is an unofficial extension not defined
// by the OAC specification.
type V1Alpha1Spec struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	Inference    *InferenceSpec           `json:"inference,omitempty"`
	MCP          map[string]MCPSpec       `json:"mcp,omitempty"`
	Workspaces   map[string]WorkspaceSpec `json:"workspace,omitempty"`
	Orchestrator *OrchestratorSpec        `json:"orchestrator,omitempty"`
	Events       map[string]EventSpec     `json:"events,omitempty"`
}

// V1Alpha2Spec is the spec for OAC images declaring [VersionV1Alpha2].
// It extends V1Alpha1Spec with session isolation support.
// When Session.Isolation is true, the Workspace map must be empty;
// [check.Check] reports a SeverityError when both are set.
type V1Alpha2Spec struct {
	V1Alpha1Spec

	// Session configures per-session isolation. When Session.Isolation is true,
	// Workspace must be empty; [check.Check] reports a SeverityError when combined.
	Session SessionSpec `json:"session"`
}

// V1Alpha3Spec is the spec for OAC images declaring [VersionV1Alpha3].
// It replaces the inference type model-ID list with structured capability and
// performance requirements; all other label groups are unchanged from v1alpha2.
type V1Alpha3Spec struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	Inference    *InferenceV3Spec         `json:"inference,omitempty"`
	MCP          map[string]MCPSpec       `json:"mcp,omitempty"`
	Workspaces   map[string]WorkspaceSpec `json:"workspace,omitempty"`
	Orchestrator *OrchestratorSpec        `json:"orchestrator,omitempty"`
	Events       map[string]EventSpec     `json:"events,omitempty"`
	Session      SessionSpec              `json:"session"`
}

// SessionSpec describes per-session runtime isolation settings.
type SessionSpec struct {
	// Isolation, when true, requests that the orchestrator provision a fresh ephemeral
	// workspace per session rather than sharing volumes across sessions.
	// Incompatible with declared workspaces; [check.Check] reports a SeverityError when combined.
	Isolation bool `json:"isolation,omitempty"`
}

// CredentialTarget describes where a credential value is delivered at runtime.
// Env is the name of an environment variable; File is an absolute filesystem path.
// The orchestrator injects the credential value into whichever source(s) are declared.
// Setting both allows fallback: the runtime checks Env first, then File.
// At least one of Env or File must be non-empty; this is enforced by [check.Check], not [Parse].
type CredentialTarget struct {
	Env  string `json:"env,omitempty"`
	File string `json:"file,omitempty"`
}

// InferenceSpec describes the agent's inference configuration for v1alpha1/v1alpha2 images.
// Types is populated by a custom UnmarshalJSON and holds per-type model lists.
type InferenceSpec struct {
	APIBase *CredentialTarget            `json:"api_base,omitempty"`
	APIKey  *CredentialTarget            `json:"api_key,omitempty"`
	Types   map[string]InferenceTypeSpec `json:"-"` // populated by UnmarshalJSON
}

// InferenceTypeSpec holds the model list for a single inference type in v1alpha1/v1alpha2 images.
type InferenceTypeSpec struct {
	// Models is a pre-split list of model identifiers accepted by this inference type.
	// Populated by UnmarshalJSON; the label value is space-separated, e.g. "gpt-4o llama-3.1-8b-instruct".
	Models []string `json:"models"` // populated by UnmarshalJSON
}

// InferenceV3Spec describes the agent's inference configuration for v1alpha3 images.
// Types holds per-type capability requirements; model selection is delegated to the orchestrator.
type InferenceV3Spec struct {
	APIBase *CredentialTarget               `json:"api_base,omitempty"`
	APIKey  *CredentialTarget               `json:"api_key,omitempty"`
	Types   map[string]InferenceTypeReqSpec `json:"-"` // populated by UnmarshalJSON
}

// InferenceTypeReqSpec holds capability and performance requirements for a single inference
// type in v1alpha3 images. The orchestrator selects from its available models the best fit
// satisfying all declared requirements.
type InferenceTypeReqSpec struct {
	// Context is the minimum context window in tokens. Zero means no constraint.
	// Populated by UnmarshalJSON; the label value is a positive integer string, e.g. "128000".
	Context int `json:"context,omitempty"`
	// Reasoning requires extended reasoning/thinking mode support when true.
	Reasoning bool `json:"reasoning,omitempty"`
	// Tools requires function/tool calling support when true.
	Tools bool `json:"tools,omitempty"`
	// Input declares required input modality capabilities.
	Input *InferenceTypeInputSpec `json:"input,omitempty"`
	// Output declares required output modality capabilities.
	Output *InferenceTypeOutputSpec `json:"output,omitempty"`
	// Bench declares minimum benchmark scores using an open vocabulary.
	// Keys are benchmark identifiers as defined by their publishers (e.g. "gpqa", "humaneval").
	// Values are decimal strings in [0, 100] representing percentage correct.
	Bench map[string]string `json:"bench,omitempty"`
}

// InferenceTypeInputSpec declares required input modality capabilities.
type InferenceTypeInputSpec struct {
	// Vision requires image input support when true.
	Vision bool `json:"vision,omitempty"`
	// Audio requires audio input support when true.
	Audio bool `json:"audio,omitempty"`
	// Video requires video input support when true.
	Video bool `json:"video,omitempty"`
}

// InferenceTypeOutputSpec declares required output modality capabilities.
type InferenceTypeOutputSpec struct {
	// Image requires image generation output support when true.
	Image bool `json:"image,omitempty"`
	// Audio requires audio/speech output support when true.
	Audio bool `json:"audio,omitempty"`
	// Video requires video generation output support when true.
	Video bool `json:"video,omitempty"`
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
	Token CredentialTarget `json:"token"`
}

// MCPOAuthAuth carries OAuth2 client credentials for an MCP server.
type MCPOAuthAuth struct {
	ClientID     CredentialTarget `json:"client_id"`
	ClientSecret CredentialTarget `json:"client_secret"`
}

// MCPDCRAuth carries Dynamic Client Registration credentials for an MCP server.
type MCPDCRAuth struct {
	// Scopes is the pre-split list of OAuth scopes to request during Dynamic Client Registration.
	// Populated by UnmarshalJSON; the label value is space-separated, e.g. "repo:read repo:write".
	Scopes       []string         `json:"scopes"` // populated by UnmarshalJSON
	ClientID     CredentialTarget `json:"client_id"`
	ClientSecret CredentialTarget `json:"client_secret"`
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
	Token CredentialTarget `json:"token"`
}

// OrchestratorMTLSAuth carries mTLS credentials for orchestrator authentication.
// Cert and Key are required credential sources. CA is optional — the orchestrator provisions
// the CA certificate; [check.Check] does not warn when CA is absent.
type OrchestratorMTLSAuth struct {
	Cert CredentialTarget `json:"cert"`
	Key  CredentialTarget `json:"key"`
	CA   CredentialTarget `json:"ca"`
}

// EventSpec describes an event subscription with an embedded schema.
type EventSpec struct {
	// Schema describes the location and format of the event payload schema
	// embedded in the container image.
	Schema EventSchema `json:"schema"`
}

// EventSchema holds the path and MIME type of the event schema file.
type EventSchema struct {
	// Path is an absolute path inside the container image to the schema file.
	Path string `json:"path,omitempty"`
	// MIMEType is the MIME type of the schema file, e.g. "application/json" or "application/yaml".
	MIMEType string `json:"mimetype,omitempty"`
}

// Image is an OAC-conformant OCI image returned by registry discovery.
// It embeds Manifest for direct field promotion (Name(), Description(), SpecVersion).
type Image struct {
	Manifest

	// Labels contains all OCI image config labels unfiltered — OAC and non-OAC alike.
	Labels map[string]string `json:"labels"`
	// Reference is the fully-qualified image reference (e.g. "registry.example.com/repo:tag")
	// as seen during the discovery scan.
	Reference string `json:"reference"`
}

// Dockerfile is an OAC-conformant Dockerfile parsed from the local filesystem via the
// dockerfile package. It embeds Manifest for direct field promotion.
type Dockerfile struct {
	Manifest

	// Path is the absolute path to the source Dockerfile that was parsed.
	Path string `json:"path"`
}
