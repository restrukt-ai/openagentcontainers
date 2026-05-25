package search_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/search"
)

// ExampleSearch_allAgents shows using an empty query to enumerate all agents
// without filtering.
func ExampleSearch_allAgents() {
	opts := discovery.NewOptions(
		discovery.WithConcurrency(4),
		discovery.WithMaxRetries(3),
		discovery.WithLimiter(rate.NewLimiter(rate.Every(time.Second), 10)),
	)

	agents, err := search.Search(context.Background(), "registry.example.com", "", opts)
	if err != nil {
		log.Fatal(err)
	}

	for _, a := range agents {
		fmt.Println(a.Name())
	}
}

// ExampleSearch shows searching a registry for agents matching a query string.
func ExampleSearch() {
	opts := discovery.NewOptions(
		discovery.WithConcurrency(4),
		discovery.WithMaxRetries(3),
		discovery.WithLimiter(rate.NewLimiter(rate.Every(time.Second), 10)),
	)

	agents, err := search.Search(context.Background(), "registry.example.com", "code", opts)
	if err != nil {
		log.Fatal(err)
	}

	for _, a := range agents {
		fmt.Println(a.Name())
	}
}
