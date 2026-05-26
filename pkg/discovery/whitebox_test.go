package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

// errServer returns a server that responds 500 to every request.
func errServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// tagServer returns a server that:
//   - serves the given tags at /v2/repo/tags/list
//   - responds 200 to HEAD /manifests (for crane.Digest) with a stub digest
//   - responds 500 to GET /manifests (causing crane.Config to fail)
func tagServer(t *testing.T, tags []string) *httptest.Server {
	t.Helper()

	type tagList struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	raw, err := json.Marshal(tagList{Name: "repo", Tags: tags})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)

		case strings.HasSuffix(r.URL.Path, "/tags/list"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(raw) //nolint:errcheck

		case strings.Contains(r.URL.Path, "/manifests/"):
			if r.Method == http.MethodHead {
				w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("a", 64))
				w.Header().
					Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// ociServer returns a server that serves a minimal OCI manifest whose config
// blob contains configBytes. The digest in the manifest matches the actual
// content hash so that crane's integrity check passes.
func ociServer(t *testing.T, repo, tag string, configBytes []byte) *httptest.Server {
	t.Helper()

	configH := sha256.Sum256(configBytes)
	configDigest := "sha256:" + hex.EncodeToString(configH[:])

	const manifestFmt = `{"schemaVersion":2,` +
		`"mediaType":"application/vnd.docker.distribution.manifest.v2+json",` +
		`"config":{"mediaType":"application/vnd.docker.container.image.v1+json",` +
		`"size":%d,"digest":%q},"layers":[]}`

	manifest := fmt.Sprintf(manifestFmt, len(configBytes), configDigest)

	manifestH := sha256.Sum256([]byte(manifest))
	manifestDigest := "sha256:" + hex.EncodeToString(manifestH[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == fmt.Sprintf("/v2/%s/manifests/%s", repo, tag):
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Header().Set("Docker-Content-Digest", manifestDigest)

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(manifest)) //nolint:errcheck
			}

		case r.URL.Path == fmt.Sprintf("/v2/%s/blobs/%s", repo, configDigest):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(configBytes) //nolint:errcheck

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func hostOf(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "http://")
}

// ──────────────────────────────────────────────
// inspectImage tests
// ──────────────────────────────────────────────

// TestInspectImageCraneConfigError verifies inspectImage returns an error when crane.Config fails.
func TestInspectImageCraneConfigError(t *testing.T) {
	t.Parallel()
	srv := errServer(t)

	_, ok, err := inspectImage(hostOf(srv)+"/repo:latest", crane.Insecure)
	if err == nil {
		t.Fatal("expected error")
	}

	if ok {
		t.Fatal("expected ok=false")
	}
}

// TestInspectImageUnmarshalError verifies inspectImage returns an error when the
// config blob contains valid JSON that cannot unmarshal into imageConfig (e.g., a JSON array).
func TestInspectImageUnmarshalError(t *testing.T) {
	t.Parallel()
	// [1,2,3] is valid JSON but not an object → json.Unmarshal into imageConfig fails.
	srv := ociServer(t, "repo", "latest", []byte("[1,2,3]"))

	_, ok, err := inspectImage(hostOf(srv)+"/repo:latest", crane.Insecure)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}

	if ok {
		t.Fatal("expected ok=false")
	}
}

// ──────────────────────────────────────────────
// scanRepo tests
// ──────────────────────────────────────────────

// TestScanRepoListTagsError verifies scanRepo exits silently when listing tags fails.
func TestScanRepoListTagsError(t *testing.T) {
	t.Parallel()
	srv := errServer(t)

	results := make(chan oac.Image, 10)

	scanRepo(context.Background(), hostOf(srv)+"/repo", 1, false, nil,
		rate.NewLimiter(rate.Inf, 0), results, crane.Insecure)

	close(results)

	for range results {
		t.Fatal("expected no agents on ListTags error")
	}
}

// TestScanRepoInspectImageErrorEarlyExit covers the path where inspectImage
// returns an error on the first tag (i=0) with force=false, causing an early return.
func TestScanRepoInspectImageErrorEarlyExit(t *testing.T) {
	t.Parallel()
	// Tags list succeeds; crane.Config fails (HEAD ok, GET 500).
	srv := tagServer(t, []string{"latest", "v1.0"})

	results := make(chan oac.Image, 10)

	scanRepo(context.Background(), hostOf(srv)+"/repo", 1, false, nil,
		rate.NewLimiter(rate.Inf, 0), results, crane.Insecure)

	close(results)

	for range results {
		t.Fatal("expected no agents when inspectImage errors on i=0 without force")
	}
}

// TestScanRepoInspectImageErrorForce covers the `continue` path when inspectImage
// returns an error but force=true suppresses the early return.
func TestScanRepoInspectImageErrorForce(t *testing.T) {
	t.Parallel()
	srv := tagServer(t, []string{"latest"})

	results := make(chan oac.Image, 10)

	scanRepo(context.Background(), hostOf(srv)+"/repo", 1, true, nil,
		rate.NewLimiter(rate.Inf, 0), results, crane.Insecure)

	close(results)

	for range results {
		t.Fatal("expected no agents when all inspections fail")
	}
}

// ──────────────────────────────────────────────
// stubCache — minimal Cache for whitebox tests
// ──────────────────────────────────────────────

type stubCache struct {
	digests    map[string][]byte
	repoLatest map[string]string
}

func (c *stubCache) GetDigest(d string) ([]byte, bool) {
	v, ok := c.digests[d]

	return v, ok
}

func (c *stubCache) SetDigest(d string, b []byte) { c.digests[d] = b }

func (c *stubCache) GetLatestDigest(r string) (string, bool) {
	v, ok := c.repoLatest[r]

	return v, ok
}

func (c *stubCache) SetLatestDigest(r, d string) { c.repoLatest[r] = d }
func (c *stubCache) Save() error                 { return nil }

// ──────────────────────────────────────────────
// handleLatestTag / handleCacheHit / shouldSkipNonOACLatest / emitAgent
// ──────────────────────────────────────────────

// TestHandleLatestTagForceWithCache verifies that force=true writes the digest
// to the cache and returns false (do not skip the repo).
func TestHandleLatestTagForceWithCache(t *testing.T) {
	t.Parallel()

	c := &stubCache{
		digests:    make(map[string][]byte),
		repoLatest: make(map[string]string),
	}
	rs := repoScanner{c: c, force: true}

	got := rs.handleLatestTag("myrepo", "sha256:abc")
	if got {
		t.Fatal("expected false (repo not skipped)")
	}

	if c.repoLatest["myrepo"] != "sha256:abc" {
		t.Fatalf("expected cache entry sha256:abc, got %q", c.repoLatest["myrepo"])
	}
}

// TestHandleCacheHitMalformedJSON verifies that a malformed cached entry returns
// tagContinue without emitting anything.
func TestHandleCacheHitMalformedJSON(t *testing.T) {
	t.Parallel()

	c := &stubCache{
		digests:    map[string][]byte{"sha256:abc": []byte("{invalid")},
		repoLatest: make(map[string]string),
	}
	out := make(chan oac.Image, 1)

	action := handleCacheHit(context.Background(), c, "sha256:abc", "ref", out)
	if action != tagContinue {
		t.Fatalf("expected tagContinue, got %v", action)
	}

	if len(out) != 0 {
		t.Fatal("expected nothing emitted for malformed JSON")
	}
}

// TestShouldSkipNonOACLatestCachedOACAgent verifies that a non-nil cached entry
// (OAC agent) does NOT cause the repo to be skipped.
func TestShouldSkipNonOACLatestCachedOACAgent(t *testing.T) {
	t.Parallel()

	c := &stubCache{
		digests:    map[string][]byte{"sha256:abc": []byte(`{"name":"agent"}`)},
		repoLatest: make(map[string]string),
	}

	got := shouldSkipNonOACLatest(c, "sha256:abc")
	if got {
		t.Fatal("should NOT skip when cached result is non-nil (OAC agent)")
	}
}

// TestEmitAgentContextCancelled verifies emitAgent returns true when the context
// is already cancelled and the output channel is unbuffered (nobody reading).
func TestEmitAgentContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := make(chan oac.Image) // unbuffered, nobody reading

	got := emitAgent(ctx, oac.Image{}, out)
	if !got {
		t.Fatal("expected true (ctx.Done() path taken)")
	}
}
