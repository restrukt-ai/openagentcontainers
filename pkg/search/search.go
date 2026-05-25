// Package search provides text-based filtering of discovered OAC agents.
package search

import (
	"context"
	"strings"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
)

// Search discovers all OAC-conformant images in registry and returns those
// whose name, version, description, or any label value contains query
// (case-insensitive substring match). An empty query returns all agents.
func Search(
	ctx context.Context,
	registry, query string,
	opts discovery.Options,
) ([]discovery.AgentImage, error) {
	agents, err := discovery.Discover(ctx, registry, opts)
	if err != nil {
		return nil, err
	}

	return filterAgents(agents, query), nil
}

// filterAgents returns agents that match query across name, version, description,
// and label values. An empty query passes all agents through.
func filterAgents(agents []discovery.AgentImage, query string) []discovery.AgentImage {
	if query == "" {
		return agents
	}

	q := strings.ToLower(query)

	var out []discovery.AgentImage

	for _, a := range agents {
		if agentMatchesQuery(a, q) {
			out = append(out, a)
		}
	}

	return out
}

// agentMatchesQuery returns true if the lowercased query is a substring of the
// agent's name, version, description, or any label value.
func agentMatchesQuery(a discovery.AgentImage, query string) bool {
	if strings.Contains(strings.ToLower(a.Name), query) {
		return true
	}

	if strings.Contains(strings.ToLower(a.Version), query) {
		return true
	}

	if strings.Contains(strings.ToLower(a.Description), query) {
		return true
	}

	for _, v := range a.Labels {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}

	return false
}
