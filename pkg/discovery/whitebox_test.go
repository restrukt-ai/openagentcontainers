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

	results := make(chan AgentImage, 10)

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

	results := make(chan AgentImage, 10)

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

	results := make(chan AgentImage, 10)

	scanRepo(context.Background(), hostOf(srv)+"/repo", 1, true, nil,
		rate.NewLimiter(rate.Inf, 0), results, crane.Insecure)

	close(results)

	for range results {
		t.Fatal("expected no agents when all inspections fail")
	}
}
