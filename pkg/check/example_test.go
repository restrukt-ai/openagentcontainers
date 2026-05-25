package check_test

import (
	"fmt"
	"log"

	"github.com/restrukt-ai/openagentcontainers/pkg/check"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func ExampleCheck_error() {
	// An MCP server declared with no credential source produces a SeverityError.
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.description":                   "does things",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		"org.openagentcontainers.mcp.calendar.bearer.token.env": "", // empty — no source
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	issues := check.Check(m)
	for _, issue := range issues {
		fmt.Printf("%s: %s: %s\n", issue.Severity, issue.Field, issue.Message)
	}
	// Output:
	// error: mcp.calendar.bearer.token: neither env nor file set; no credential source
}

func ExampleCheck() {
	labels := map[string]string{
		"org.openagentcontainers.version":                       "v1alpha1",
		"org.openagentcontainers.name":                          "my-agent",
		"org.openagentcontainers.orchestrator.env":              "ORCHESTRATOR_URL",
		"org.openagentcontainers.orchestrator.bearer.token.env": "ORCHESTRATOR_TOKEN",
		// no description set — expect a warning
	}

	m, err := oac.Parse(labels)
	if err != nil {
		log.Fatal(err)
	}

	issues := check.Check(m)

	for _, issue := range issues {
		fmt.Printf("%s: %s: %s\n", issue.Severity, issue.Field, issue.Message)
	}
	// Output:
	// warning: description: description is not set
}
