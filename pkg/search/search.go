// Package search wraps [discovery.Discover] with a case-insensitive substring filter.
//
// Use [Search] to enumerate a registry and narrow results by name, version, description,
// or any label value. Pass an empty query to return all agents.
//
// Construct opts with [discovery.NewOptions] exactly as you would for a direct
// [discovery.Discover] call — the same rate-limiter, concurrency, and cache settings apply.
package search

import (
	"context"
	"strings"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
)

// Search discovers all OAC-conformant images in registry and returns those
// whose name, version, description, or any label value contains query
// (case-insensitive substring match). registry is the hostname of the OCI registry
// to scan (e.g. "registry.example.com"). query is the case-insensitive substring
// to match; pass an empty string to return all agents.
//
// Any error returned by [discovery.Discover] (network failure, context cancellation, etc.) is
// returned unchanged; in that case the returned slice is nil.
//
// Note: the label search covers all OCI image labels, not just OAC-prefixed ones. Non-OAC
// labels (e.g. org.opencontainers.image.created) may produce unexpected matches.
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
