package oac_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func ExampleParse() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.SpecVersion)
	fmt.Println(m.V1Alpha1.Name)
	// Output:
	// v1alpha1
	// my-agent
}

func ExampleParse_unknownVersion() {
	labels := map[string]string{
		"org.openagentcontainers.version": "v99",
	}
	_, err := oac.Parse(labels)
	fmt.Println(errors.Is(err, oac.ErrUnsupportedVersion))
	// Output:
	// true
}

func ExampleV1Alpha1Spec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.description":                   "does things",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha1.Name)
	fmt.Println(m.V1Alpha1.Description)
	// Output:
	// my-agent
	// does things
}

func ExampleV1Alpha2Spec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha2",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.session.isolation":             "true",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha2.Name)
	fmt.Println(m.V1Alpha2.Session.Isolation)
	// Output:
	// my-agent
	// true
}

func ExampleSessionSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha2",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.session.isolation":             "true",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha2.Session.Isolation)
	// Output:
	// true
}

func ExampleCredentialTarget() {
	// Both env and file can be set simultaneously as a fallback pair.
	labels := map[string]string{
		"org.openagentcontainers.version":                        "v1alpha1",
		"org.openagentcontainers.name":                           "my-agent",
		"org.openagentcontainers.orchestrator.env":               "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env":  "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.orchestrator.bearer.token.file": "/run/secrets/token",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	token := m.V1Alpha1.Orchestrator.Bearer.Token
	fmt.Println(token.Env)
	fmt.Println(token.File)
	// Output:
	// ORCHESTRATOR_TOKEN
	// /run/secrets/token
}

func ExampleInferenceSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.inference.api_base.env":        "INFERENCE_BASE_URL",
		"org.openagentcontainers.inference.api_key.env":         "INFERENCE_API_KEY",
		"org.openagentcontainers.inference.chat.models":         "gpt-4o gpt-4o-mini",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	inf := m.V1Alpha1.Inference
	fmt.Println(inf.APIBase.Env)
	fmt.Println(inf.Types["chat"].Models)
	// Output:
	// INFERENCE_BASE_URL
	// [gpt-4o gpt-4o-mini]
}

func ExampleInferenceTypeSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.inference.chat.models":         "gpt-4o llama-3.1-8b-instruct",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha1.Inference.Types["chat"].Models)
	// Output:
	// [gpt-4o llama-3.1-8b-instruct]
}

func ExampleMCPSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.mcp.calendar.bearer.token.env": "CALENDAR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	spec := m.V1Alpha1.MCP["calendar"]
	fmt.Println(spec.Bearer != nil)
	fmt.Println(spec.OAuth == nil)
	fmt.Println(spec.DCR == nil)
	// Output:
	// true
	// true
	// true
}

func ExampleMCPBearerAuth() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.mcp.calendar.bearer.token.env": "CALENDAR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha1.MCP["calendar"].Bearer.Token.Env)
	// Output:
	// CALENDAR_TOKEN
}

func ExampleMCPOAuthAuth() {
	labels := map[string]string{
		"org.openagentcontainers.version":                              "v1alpha1",
		"org.openagentcontainers.name":                                 "my-agent",
		"org.openagentcontainers.orchestrator.env":                     "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env":        "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.mcp.calendar.oauth.client_id.env":     "CALENDAR_CLIENT_ID",
		"org.openagentcontainers.mcp.calendar.oauth.client_secret.env": "CALENDAR_CLIENT_SECRET",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	oauth := m.V1Alpha1.MCP["calendar"].OAuth
	fmt.Println(oauth.ClientID.Env)
	fmt.Println(oauth.ClientSecret.Env)
	// Output:
	// CALENDAR_CLIENT_ID
	// CALENDAR_CLIENT_SECRET
}

func ExampleMCPDCRAuth() {
	labels := map[string]string{
		"org.openagentcontainers.version":                            "v1alpha1",
		"org.openagentcontainers.name":                               "my-agent",
		"org.openagentcontainers.orchestrator.env":                   "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env":      "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.mcp.calendar.dcr.scopes":            "calendar:read calendar:write",
		"org.openagentcontainers.mcp.calendar.dcr.client_id.env":     "CALENDAR_CLIENT_ID",
		"org.openagentcontainers.mcp.calendar.dcr.client_secret.env": "CALENDAR_CLIENT_SECRET",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	dcr := m.V1Alpha1.MCP["calendar"].DCR
	fmt.Println(dcr.Scopes)
	fmt.Println(dcr.ClientID.Env)
	// Output:
	// [calendar:read calendar:write]
	// CALENDAR_CLIENT_ID
}

func ExampleWorkspaceSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.workspace.data.path":           "/data",
		"org.openagentcontainers.workspace.data.mutable":        "true",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	ws := m.V1Alpha1.Workspaces["data"]
	fmt.Println(ws.Path)
	fmt.Println(ws.Mutable)
	// Output:
	// /data
	// true
}

func ExampleOrchestratorSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	orch := m.V1Alpha1.Orchestrator
	fmt.Println(orch.Env)
	fmt.Println(orch.Bearer.Token.Env)
	// Output:
	// ORCHESTRATOR_URL
	// ORCHESTRATOR_TOKEN
}

func ExampleOrchestratorBearerAuth() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(m.V1Alpha1.Orchestrator.Bearer.Token.Env)
	// Output:
	// ORCHESTRATOR_TOKEN
}

func ExampleOrchestratorMTLSAuth() {
	labels := map[string]string{
		"org.openagentcontainers.version":                     "v1alpha1",
		"org.openagentcontainers.name":                        "my-agent",
		"org.openagentcontainers.orchestrator.env":            "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.mtls.cert.file": "/run/secrets/tls.crt",
		"org.openagentcontainers.orchestrator.mtls.key.file":  "/run/secrets/tls.key",
		"org.openagentcontainers.orchestrator.mtls.ca.file":   "/run/secrets/ca.crt",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	mtls := m.V1Alpha1.Orchestrator.MTLS
	fmt.Println(mtls.Cert.File)
	fmt.Println(mtls.Key.File)
	fmt.Println(mtls.CA.File)
	// Output:
	// /run/secrets/tls.crt
	// /run/secrets/tls.key
	// /run/secrets/ca.crt
}

func ExampleEventSpec() {
	labels := map[string]string{
		"org.openagentcontainers.version":                             "v1alpha1",
		"org.openagentcontainers.name":                                "my-agent",
		"org.openagentcontainers.orchestrator.env":                    "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env":       "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.events.order-placed.schema.path":     "/schemas/order-placed.json",
		"org.openagentcontainers.events.order-placed.schema.mimetype": "application/json",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(m.V1Alpha1.Events))
	fmt.Println(m.V1Alpha1.Events["order-placed"].Schema.Path)
	// Output:
	// 1
	// /schemas/order-placed.json
}

func ExampleEventSchema() {
	labels := map[string]string{
		"org.openagentcontainers.version":                             "v1alpha1",
		"org.openagentcontainers.name":                                "my-agent",
		"org.openagentcontainers.orchestrator.env":                    "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env":       "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.events.order-placed.schema.path":     "/schemas/order-placed.json",
		"org.openagentcontainers.events.order-placed.schema.mimetype": "application/json",
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	schema := m.V1Alpha1.Events["order-placed"].Schema
	fmt.Println(schema.Path)
	fmt.Println(schema.MIMEType)
	// Output:
	// /schemas/order-placed.json
	// application/json
}
