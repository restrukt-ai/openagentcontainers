package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// errSaveCacheDiskFull is a static sentinel used by TestSaveCache_Error.
var errSaveCacheDiskFull = errors.New("disk full")

// --- mockCache ---

type mockCache struct{ saveErr error }

func (m *mockCache) GetDigest(string) ([]byte, bool)       { return nil, false }
func (m *mockCache) SetDigest(string, []byte)              {}
func (m *mockCache) GetLatestDigest(string) (string, bool) { return "", false }
func (m *mockCache) SetLatestDigest(_, _ string)           {}
func (m *mockCache) Save() error                           { return m.saveErr }

// --- helpers ---

func makeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	return cmd
}

func testRegistry(t *testing.T) (string, []crane.Option) {
	t.Helper()

	srv := httptest.NewServer(gcrregistry.New())
	t.Cleanup(srv.Close)

	return strings.TrimPrefix(srv.URL, "http://"), []crane.Option{crane.Insecure}
}

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

func push(t *testing.T, img v1.Image, ref string, opts []crane.Option) {
	t.Helper()

	err := crane.Push(img, ref, opts...)
	if err != nil {
		t.Fatalf("push %s: %v", ref, err)
	}
}

// --- saveCache ---

func TestSaveCache_Nil(t *testing.T) {
	t.Parallel()

	saveCache(nil) // must not panic
}

func TestSaveCache_Success(t *testing.T) {
	t.Parallel()

	saveCache(&mockCache{saveErr: nil}) // must not panic
}

func TestSaveCache_Error(t *testing.T) { //nolint:paralleltest // redirects os.Stderr
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stderr
	os.Stderr = w

	saveCache(&mockCache{saveErr: errSaveCacheDiskFull})

	w.Close()

	os.Stderr = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	assert.Contains(t, buf.String(), "warning: save cache")
	assert.Contains(t, buf.String(), "disk full")
}

// --- buildOpts ---

func TestBuildOpts_NoCacheTrue(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 1}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.Nil(t, opts.Cache())
}

func TestBuildOpts_ExplicitCachePath(t *testing.T) {
	t.Parallel()

	f := commonFlags{
		noCache:     false,
		cachePath:   filepath.Join(t.TempDir(), "c.json"),
		rateLimit:   10,
		concurrency: 1,
	}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.NotNil(t, opts.Cache())
}

func TestBuildOpts_DefaultCachePath(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: false, cachePath: "", rateLimit: 10, concurrency: 1}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.NotNil(t, opts.Cache())
}

func TestBuildOpts_RateLimitZero(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 0, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

func TestBuildOpts_Insecure(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, insecure: true, rateLimit: 10, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

func TestBuildOpts_Force(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, force: true, rateLimit: 10, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

// --- writeAgentsTable ---

func TestWriteAgentsTable_Empty(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	err := writeAgentsTable(nil)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "REFERENCE")
}

func TestWriteAgentsTable_WithData(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	agents := []discovery.AgentImage{
		{
			Manifest: oac.Manifest{
				SpecVersion: oac.VersionV1Alpha1,
				V1Alpha1:    &oac.V1Alpha1Spec{Name: "my-agent"},
			},
			Reference: "reg/my-agent:latest",
		},
	}
	err := writeAgentsTable(agents)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "my-agent")
	assert.Contains(t, buf.String(), "reg/my-agent:latest")
}

// --- runDiscover ---

func TestRunDiscover_JSONOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "test-agent",
	})
	push(t, img, host+"/test-agent:latest", craneOpts)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := commonFlags{outputJSON: true, noCache: true, rateLimit: 10, concurrency: 2}
	err := runDiscover(makeCmd(), []string{host}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)

	var agents []discovery.AgentImage
	require.NoError(t, json.NewDecoder(&buf).Decode(&agents))
	assert.GreaterOrEqual(t, len(agents), 1)
}

func TestRunDiscover_TableOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "test-agent",
	})
	push(t, img, host+"/test-agent:latest", craneOpts)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := commonFlags{outputJSON: false, noCache: true, rateLimit: 10, concurrency: 2}
	err := runDiscover(makeCmd(), []string{host}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "REFERENCE")
	assert.Contains(t, buf.String(), host)
}

func TestRunDiscover_Error(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 2}
	err := runDiscover(makeCmd(), []string{"localhost:1"}, f)
	require.Error(t, err)
}

// --- runSearch ---

func TestRunSearch_NoResults(t *testing.T) { //nolint:paralleltest // redirects os.Stderr
	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stderr
	os.Stderr = w

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(makeCmd(), []string{host, "zzz"}, f)

	w.Close()

	os.Stderr = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No agents found")
}

func TestRunSearch_JSONOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := commonFlags{outputJSON: true, noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(makeCmd(), []string{host, "code"}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)

	var agents []discovery.AgentImage
	require.NoError(t, json.NewDecoder(&buf).Decode(&agents))
	assert.Len(t, agents, 1)
}

func TestRunSearch_TableOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := commonFlags{outputJSON: false, noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(makeCmd(), []string{host, "code"}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "REFERENCE")
}

func TestRunSearch_Error(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(makeCmd(), []string{"localhost:1", "query"}, f)
	require.Error(t, err)
}
