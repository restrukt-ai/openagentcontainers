package search_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
	"github.com/restrukt-ai/openagentcontainers/pkg/search"
)

func integTestRegistry(t *testing.T) (string, []crane.Option) {
	t.Helper()

	srv := httptest.NewServer(gcrregistry.New())
	t.Cleanup(srv.Close)

	return strings.TrimPrefix(srv.URL, "http://"), []crane.Option{crane.Insecure}
}

func integMakeImage(t *testing.T, labels map[string]string) v1.Image {
	t.Helper()

	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Config: v1.Config{Labels: labels},
	})
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func integPush(t *testing.T, img v1.Image, ref string, opts []crane.Option) {
	t.Helper()

	err := crane.Push(img, ref, opts...)
	if err != nil {
		t.Fatalf("push %s: %v", ref, err)
	}
}

func searchOpts(craneOpts []crane.Option) discovery.Options {
	return discovery.NewOptions(
		discovery.WithConcurrency(2),
		discovery.WithMaxRetries(1),
		discovery.WithLimiter(rate.NewLimiter(rate.Inf, 0)),
		discovery.WithCraneOpts(craneOpts...),
	)
}

// TestSearch_ContextCancelled verifies Search propagates context cancellation from Discover.
func TestSearch_ContextCancelled(t *testing.T) {
	t.Parallel()

	host, craneOpts := integTestRegistry(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := search.Search(ctx, host, "", searchOpts(craneOpts))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestSearch_EmptyQueryReturnsAll verifies an empty query returns all discovered agents.
func TestSearch_EmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()

	host, craneOpts := integTestRegistry(t)

	integPush(t, integMakeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "agent-a",
	}), host+"/agent-a:latest", craneOpts)

	integPush(t, integMakeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha2),
		oac.LabelName:    "agent-b",
	}), host+"/agent-b:latest", craneOpts)

	agents, err := search.Search(context.Background(), host, "", searchOpts(craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
}

// TestSearch_QueryFilters verifies a non-empty query filters agents by name.
func TestSearch_QueryFilters(t *testing.T) {
	t.Parallel()

	host, craneOpts := integTestRegistry(t)

	integPush(t, integMakeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	}), host+"/code-agent:latest", craneOpts)

	integPush(t, integMakeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha2),
		oac.LabelName:    "data-agent",
	}), host+"/data-agent:latest", craneOpts)

	agents, err := search.Search(context.Background(), host, "code", searchOpts(craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 1 || agents[0].Name() != "code-agent" {
		t.Fatalf("expected 1 agent named code-agent, got %v", agents)
	}
}
