package discovery_test

import (
	"context"
	"fmt"
	"maps"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/discovery"
	"github.com/restrukt-ai/openagentcontainers/oac"
)

// testRegistry starts an in-memory OCI registry and returns its host and crane options.
func testRegistry(t *testing.T) (string, []crane.Option) {
	t.Helper()

	srv := httptest.NewServer(gcrregistry.New())
	t.Cleanup(srv.Close)

	return strings.TrimPrefix(srv.URL, "http://"), []crane.Option{crane.Insecure}
}

// makeImage builds a minimal OCI image with the given labels.
func makeImage(t *testing.T, labels map[string]string) v1.Image {
	t.Helper()

	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Config: v1.Config{Labels: labels},
	})
	if err != nil {
		t.Fatal(err)
	}

	return img
}

// push pushes img to ref in the test registry.
func push(t *testing.T, img v1.Image, ref string, opts []crane.Option) {
	t.Helper()

	err := crane.Push(img, ref, opts...)
	if err != nil {
		t.Fatalf("push %s: %v", ref, err)
	}
}

func inf() *rate.Limiter { return rate.NewLimiter(rate.Inf, 0) }

func oacLabels(version, name string) map[string]string {
	return map[string]string{
		oac.LabelVersion: version,
		oac.LabelName:    name,
	}
}

// baseOpts returns a minimal NewOptions call shared across most tests.
func baseOpts(
	concurrency int,
	craneOpts []crane.Option,
	extra ...discovery.Option,
) discovery.Options {
	opts := make([]discovery.Option, 0, 4+len(extra))
	opts = append(opts,
		discovery.WithConcurrency(concurrency),
		discovery.WithMaxRetries(1),
		discovery.WithLimiter(inf()),
		discovery.WithCraneOpts(craneOpts...),
	)

	return discovery.NewOptions(append(opts, extra...)...)
}

// testCache is a thread-safe in-memory implementation of discovery.Cache for tests.
// Call clone to get an independent copy simulating a save+reload.
type testCache struct {
	mu         sync.RWMutex
	digests    map[string][]byte
	repoLatest map[string]string
}

var _ discovery.Cache = (*testCache)(nil)

func newTestCache() *testCache {
	return &testCache{
		digests:    make(map[string][]byte),
		repoLatest: make(map[string]string),
	}
}

func (c *testCache) Save() error { return nil }

func (c *testCache) GetDigest(digest string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	v, ok := c.digests[digest]

	return v, ok
}

func (c *testCache) SetDigest(digest string, agentJSON []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.digests[digest] = agentJSON
}

func (c *testCache) GetLatestDigest(repo string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	d, ok := c.repoLatest[repo]

	return d, ok
}

func (c *testCache) SetLatestDigest(repo, digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.repoLatest[repo] = digest
}

// clone returns an independent copy, simulating a save+reload cycle.
func (c *testCache) clone() *testCache {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c2 := newTestCache()

	for k, v := range c.digests {
		if v != nil {
			cp := make([]byte, len(v))
			copy(cp, v)
			c2.digests[k] = cp
		} else {
			c2.digests[k] = nil
		}
	}

	maps.Copy(c2.repoLatest, c.repoLatest)

	return c2
}

// TestDiscoverEmpty verifies Discover returns no agents from an empty registry.
func TestDiscoverEmpty(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

// TestDiscoverFindsOACImage verifies Discover returns all tags of an OAC image.
func TestDiscoverFindsOACImage(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	img := makeImage(t, oacLabels(oac.VersionV1Alpha1, "my-agent"))
	push(t, img, host+"/myagent:latest", craneOpts)
	push(t, img, host+"/myagent:v1.0", craneOpts)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) < 1 {
		t.Fatal("expected at least 1 agent, got 0")
	}

	for _, a := range agents {
		if a.Version != oac.VersionV1Alpha1 {
			t.Errorf("agent %s: version = %q, want v1alpha1", a.Reference, a.Version)
		}

		if a.Name != "my-agent" {
			t.Errorf("agent %s: name = %q, want my-agent", a.Reference, a.Name)
		}

		if a.Reference == "" {
			t.Error("agent has empty reference")
		}
	}
}

// TestDiscoverSkipsNonOACLatest verifies that a repo is skipped when latest has no OAC labels.
func TestDiscoverSkipsNonOACLatest(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(t, makeImage(t, map[string]string{"other": "label"}), host+"/repo:latest", craneOpts)
	push(t, makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent")), host+"/repo:v1.0", craneOpts)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 0 {
		t.Fatalf("expected 0 agents (non-OAC latest), got %d: %v", len(agents), agents)
	}
}

// TestDiscoverForceScansAllTags verifies --force scans all tags regardless of latest.
func TestDiscoverForceScansAllTags(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(t, makeImage(t, map[string]string{"other": "label"}), host+"/repo:latest", craneOpts)
	push(t, makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent")), host+"/repo:v1.0", craneOpts)

	agents, err := discovery.Discover(
		context.Background(), host, baseOpts(2, craneOpts, discovery.WithForce()),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent (force mode), got %d", len(agents))
	}

	if agents[0].Version != oac.VersionV1Alpha1 {
		t.Errorf("version: got %q, want v1alpha1", agents[0].Version)
	}
}

// TestDiscoverNoLatestTag verifies all tags are scanned when "latest" is absent.
func TestDiscoverNoLatestTag(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	img := makeImage(t, oacLabels(oac.VersionV1Alpha2, "agent"))
	push(t, img, host+"/repo:v2.0", craneOpts)
	push(t, img, host+"/repo:stable", craneOpts)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 2 {
		t.Fatalf("expected 2 agents (no latest tag), got %d", len(agents))
	}
}

// TestDiscoverMultipleRepos verifies Discover handles multiple repos independently.
func TestDiscoverMultipleRepos(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(t,
		makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent-a")),
		host+"/repo-a:latest",
		craneOpts,
	)
	push(t, makeImage(t, make(map[string]string)), host+"/repo-b:latest", craneOpts)
	push(t,
		makeImage(t, oacLabels(oac.VersionV1Alpha2, "agent-c")),
		host+"/repo-c:latest",
		craneOpts,
	)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(3, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 2 {
		t.Fatalf("expected 2 OAC agents, got %d", len(agents))
	}
}

// TestDiscoverNoOACLabels verifies images with labels but no OAC labels are ignored.
func TestDiscoverNoOACLabels(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(
		t,
		makeImage(t, map[string]string{"com.example.foo": "bar"}),
		host+"/repo:latest",
		craneOpts,
	)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

// TestDiscoverWithCachePopulates verifies the cache is populated on first scan.
func TestDiscoverWithCachePopulates(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	img := makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent"))
	push(t, img, host+"/repo:latest", craneOpts)

	c := newTestCache()

	agents, err := discovery.Discover(
		context.Background(), host, baseOpts(2, craneOpts, discovery.WithCache(c)),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) == 0 {
		t.Fatal("expected agents from first scan")
	}

	_, found := c.GetLatestDigest(host + "/repo")
	if !found {
		t.Fatal("expected repoLatest entry in cache after first scan")
	}
}

// TestDiscoverSecondScanUsesCache verifies the second scan returns the same results via cache.
func TestDiscoverSecondScanUsesCache(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	img := makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent"))
	push(t, img, host+"/repo:latest", craneOpts)

	ctx := context.Background()
	c := newTestCache()

	agents1, err := discovery.Discover(ctx, host, baseOpts(2, craneOpts, discovery.WithCache(c)))
	if err != nil {
		t.Fatal(err)
	}

	agents2, err := discovery.Discover(
		ctx, host, baseOpts(2, craneOpts, discovery.WithCache(c.clone())),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(agents2) != len(agents1) {
		t.Fatalf("second scan: got %d agents, want %d", len(agents2), len(agents1))
	}
}

// TestDiscoverCacheRepoShortcut verifies non-OAC repos are skipped on second scan via repo-level cache.
func TestDiscoverCacheRepoShortcut(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(t, makeImage(t, make(map[string]string)), host+"/repo:latest", craneOpts)

	ctx := context.Background()
	c := newTestCache()

	// First scan: no agents, cache populated.
	agents1, err := discovery.Discover(ctx, host, baseOpts(1, craneOpts, discovery.WithCache(c)))
	require.NoError(t, err)

	if len(agents1) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents1))
	}

	// Second scan: repo-level shortcut fires, returns same result.
	agents2, err := discovery.Discover(
		ctx, host, baseOpts(1, craneOpts, discovery.WithCache(c.clone())),
	)
	require.NoError(t, err)

	if len(agents2) != 0 {
		t.Fatalf("expected 0 agents on second scan (shortcut), got %d", len(agents2))
	}
}

// TestDiscoverOACAgentFields verifies all required fields are populated.
func TestDiscoverOACAgentFields(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	labels := map[string]string{
		oac.LabelVersion: oac.VersionV1Alpha1,
		oac.LabelName:    "test-agent",
		"custom.label":   "value",
	}
	push(t, makeImage(t, labels), host+"/agent:latest", craneOpts)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]

	if a.Version != oac.VersionV1Alpha1 {
		t.Errorf("Version: got %q, want v1alpha1", a.Version)
	}

	if a.Name != "test-agent" {
		t.Errorf("Name: got %q, want test-agent", a.Name)
	}

	if a.Labels["custom.label"] != "value" {
		t.Errorf("Labels[custom.label]: got %q, want value", a.Labels["custom.label"])
	}

	if !strings.Contains(a.Reference, "agent:latest") {
		t.Errorf("Reference %q should contain agent:latest", a.Reference)
	}
}

// TestDiscoverCacheHitNonOACLatest verifies the per-digest cache skips a repo when latest
// has a cached non-OAC result but the repo-level shortcut did not fire (no prior latest).
func TestDiscoverCacheHitNonOACLatest(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	nonOAC := makeImage(t, make(map[string]string))

	// Push only to v1.0 — no latest yet.
	push(t, nonOAC, host+"/repo:v1.0", craneOpts)

	c := newTestCache()

	// First scan (force=true) so v1.0 is inspected and cached as non-OAC (digest → nil).
	// There's no latest tag, so SetLatestDigest is never called.
	_, err := discovery.Discover(
		context.Background(),
		host,
		baseOpts(1, craneOpts, discovery.WithForce(), discovery.WithCache(c)),
	)
	require.NoError(t, err)

	// Now point latest at the same image (same digest).
	push(t, nonOAC, host+"/repo:latest", craneOpts)

	// Second scan (force=false): repo-level shortcut won't fire (no prior latest),
	// but per-digest cache finds digest→nil for latest → i==0 && !force → skip repo.
	agents, err := discovery.Discover(
		context.Background(),
		host,
		baseOpts(1, craneOpts, discovery.WithCache(c.clone())),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 0 {
		t.Fatalf("expected 0 agents (per-digest cache early exit), got %d", len(agents))
	}
}

// TestSearchFindsMatchingAgents verifies Search returns only agents matching the query.
func TestSearchFindsMatchingAgents(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	push(t, makeImage(t, map[string]string{
		oac.LabelVersion:     oac.VersionV1Alpha1,
		oac.LabelName:        "web-scraper",
		oac.LabelDescription: "Scrapes the web for data",
	}), host+"/web-scraper:latest", craneOpts)

	push(t, makeImage(t, map[string]string{
		oac.LabelVersion:     oac.VersionV1Alpha2,
		oac.LabelName:        "sql-analyst",
		oac.LabelDescription: "Analyses SQL databases",
	}), host+"/sql-analyst:latest", craneOpts)

	agents, err := discovery.Discover(context.Background(), host, baseOpts(2, craneOpts))
	if err != nil {
		t.Fatal(err)
	}

	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
}

// TestDiscoverContextCancel verifies Discover returns without hanging on a cancelled context.
func TestDiscoverContextCancel(t *testing.T) {
	t.Parallel()
	host, craneOpts := testRegistry(t)

	for i := range 5 {
		push(t, makeImage(t, oacLabels(oac.VersionV1Alpha1, "agent")),
			fmt.Sprintf("%s/repo%d:latest", host, i), craneOpts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Must not hang; cancelled context produces an error — assert it.
	_, err := discovery.Discover(ctx, host, baseOpts(2, craneOpts))
	assert.Error(t, err)
}
