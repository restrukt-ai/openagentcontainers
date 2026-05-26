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
		fmt.Println(a.Name(), a.SpecVersion)
	}
}

// ExampleDiscover_withCache shows how to attach a scan cache so that images seen on a
// previous run are not re-fetched. Production implementations typically persist entries
// to disk between runs; the in-memory implementation below illustrates the Cache interface.
func ExampleDiscover_withCache() {
	var cache memCache

	opts := discovery.NewOptions(
		discovery.WithConcurrency(4),
		discovery.WithMaxRetries(3),
		discovery.WithLimiter(rate.NewLimiter(rate.Every(time.Second), 10)),
		discovery.WithCache(&cache),
	)

	_, err := discovery.Discover(context.Background(), "registry.example.com", opts)
	if err != nil {
		log.Fatal(err)
	}

	// Persist entries so the next run can skip unchanged images.
	err = cache.Save()
	if err != nil {
		log.Fatal(err)
	}
}

// memCache is a minimal in-memory [discovery.Cache] implementation.
type memCache struct {
	digests map[string][]byte
	latest  map[string]string
}

func (c *memCache) GetDigest(digest string) ([]byte, bool) {
	v, ok := c.digests[digest]

	return v, ok
}

func (c *memCache) SetDigest(digest string, agentJSON []byte) {
	if c.digests == nil {
		c.digests = make(map[string][]byte)
	}

	c.digests[digest] = agentJSON
}

func (c *memCache) GetLatestDigest(repo string) (string, bool) {
	v, ok := c.latest[repo]

	return v, ok
}

func (c *memCache) SetLatestDigest(repo, digest string) {
	if c.latest == nil {
		c.latest = make(map[string]string)
	}

	c.latest[repo] = digest
}

func (c *memCache) Save() error { return nil }

// ExampleFetchLabels shows fetching raw OCI image labels for manual parsing.
func ExampleFetchLabels() {
	labels, err := discovery.FetchLabels("registry.example.com/my-agent:latest")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(labels["org.openagentcontainers.version"])
}
