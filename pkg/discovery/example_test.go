package discovery_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
)

// ExampleDiscover shows the minimal setup for scanning a private registry.
func ExampleDiscover() {
	opts := discovery.NewOptions(
		discovery.WithConcurrency(4),
		discovery.WithMaxRetries(3),
		discovery.WithLimiter(rate.NewLimiter(rate.Every(time.Second), 10)),
	)

	agents, err := discovery.Discover(context.Background(), "registry.example.com", opts)
	if err != nil {
		log.Fatal(err)
	}

	for _, a := range agents {
		fmt.Println(a.Name, a.Version)
	}
}

// ExampleFetchLabels shows fetching raw OCI image labels for manual parsing.
func ExampleFetchLabels() {
	labels, err := discovery.FetchLabels("registry.example.com/my-agent:latest")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(labels["org.openagentcontainers.version"])
}
