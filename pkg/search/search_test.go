package search

import (
	"maps"
	"testing"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

const agentSQLAnalyst = "sql-analyst"

func agent(
	name, description string,
	sv oac.SpecVersion,
	extraLabels map[string]string,
) discovery.AgentImage {
	labels := map[string]string{
		oac.LabelVersion:     string(sv),
		oac.LabelName:        name,
		oac.LabelDescription: description,
	}

	maps.Copy(labels, extraLabels)

	m, err := oac.Parse(labels)
	if err != nil {
		panic(err)
	}

	return discovery.AgentImage{
		Manifest:  *m,
		Labels:    labels,
		Reference: "reg.example.com/" + name + ":latest",
	}
}

var testAgents = []discovery.AgentImage{
	agent(
		"web-scraper",
		"Scrapes websites for data",
		oac.VersionV1Alpha1,
		map[string]string{"runtime": "python"},
	),
	agent(
		agentSQLAnalyst,
		"Analyses SQL databases",
		oac.VersionV1Alpha2,
		map[string]string{"runtime": "go"},
	),
	agent(
		"image-tagger",
		"Tags images using vision models",
		oac.VersionV1Alpha1,
		map[string]string{"runtime": "python"},
	),
	agent(
		"data-pipeline",
		"Orchestrates data pipelines",
		oac.VersionV1Alpha2,
		map[string]string{"domain": "etl"},
	),
}

func TestFilterAgentsEmptyQuery(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "")
	if len(got) != len(testAgents) {
		t.Fatalf("empty query: got %d agents, want %d", len(got), len(testAgents))
	}
}

func TestFilterAgentsMatchesName(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "sql")
	if len(got) != 1 || got[0].Name() != agentSQLAnalyst {
		t.Fatalf("name match: got %v", got)
	}
}

func TestFilterAgentsMatchesVersion(t *testing.T) {
	t.Parallel()

	// v1alpha2 appears in sql-analyst and data-pipeline
	got := filterAgents(testAgents, "v1alpha2")
	if len(got) != 2 {
		t.Fatalf("version match: got %d agents, want 2", len(got))
	}
}

func TestFilterAgentsMatchesDescription(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "vision")
	if len(got) != 1 || got[0].Name() != "image-tagger" {
		t.Fatalf("description match: got %v", got)
	}
}

func TestFilterAgentsMatchesLabelValue(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "etl")
	if len(got) != 1 || got[0].Name() != "data-pipeline" {
		t.Fatalf("label value match: got %v", got)
	}
}

func TestFilterAgentsMatchesMultiple(t *testing.T) {
	t.Parallel()
	// "python" appears as a label value in two agents
	got := filterAgents(testAgents, "python")
	if len(got) != 2 {
		t.Fatalf("multi match: got %d agents, want 2", len(got))
	}
}

func TestFilterAgentsCaseInsensitive(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "SQL")
	if len(got) != 1 || got[0].Name() != agentSQLAnalyst {
		t.Fatalf("case-insensitive: got %v", got)
	}
}

func TestFilterAgentsNoMatch(t *testing.T) {
	t.Parallel()

	got := filterAgents(testAgents, "blockchain")
	if len(got) != 0 {
		t.Fatalf("no match: got %d agents", len(got))
	}
}

func TestFilterAgentsMatchesNameSubstring(t *testing.T) {
	t.Parallel()
	// "data" appears in both "data-pipeline" name and "Scrapes websites for data" description
	got := filterAgents(testAgents, "data")
	if len(got) < 2 {
		t.Fatalf("substring match: expected ≥2, got %d", len(got))
	}
}

func TestFilterAgentsNilInput(t *testing.T) {
	t.Parallel()

	got := filterAgents(nil, "anything")
	if got != nil {
		t.Fatalf("nil input: expected nil, got %v", got)
	}
}

func TestFilterAgentsEmptyInput(t *testing.T) {
	t.Parallel()

	got := filterAgents(make([]discovery.AgentImage, 0), "anything")
	if len(got) != 0 {
		t.Fatalf("empty input: got %d agents", len(got))
	}
}

func TestAgentMatchesQueryName(t *testing.T) {
	t.Parallel()

	a := agent("my-agent", "", oac.VersionV1Alpha1, nil)
	if !agentMatchesQuery(a, "my") {
		t.Fatal("should match name")
	}
}

func TestAgentMatchesQueryVersion(t *testing.T) {
	t.Parallel()

	a := agent("agent", "", oac.VersionV1Alpha2, nil)
	if !agentMatchesQuery(a, "alpha2") {
		t.Fatal("should match version")
	}
}

func TestAgentMatchesQueryDescription(t *testing.T) {
	t.Parallel()

	a := agent("agent", "does cool things", oac.VersionV1Alpha1, nil)
	if !agentMatchesQuery(a, "cool") {
		t.Fatal("should match description")
	}
}

func TestAgentMatchesQueryLabelValue(t *testing.T) {
	t.Parallel()

	a := agent("agent", "", oac.VersionV1Alpha1, map[string]string{"env": "production"})
	if !agentMatchesQuery(a, "production") {
		t.Fatal("should match label value")
	}
}

func TestAgentMatchesQueryNoMatch(t *testing.T) {
	t.Parallel()

	a := agent("agent", "simple agent", oac.VersionV1Alpha1, nil)
	if agentMatchesQuery(a, "xyzzy") {
		t.Fatal("should not match")
	}
}

func TestAgentMatchesQueryEmptyAgent(t *testing.T) {
	t.Parallel()

	if agentMatchesQuery(discovery.AgentImage{}, "anything") {
		t.Fatal("empty agent should not match non-empty query")
	}
}
