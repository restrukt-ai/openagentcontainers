package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func TestRunDiscover_JSONOutput(t *testing.T) {
	t.Parallel()

	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "test-agent",
	})
	push(t, img, host+"/test-agent:latest", craneOpts)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := commonFlags{outputJSON: true, noCache: true, rateLimit: 10, concurrency: 2}
	err := runDiscover(cmd, []string{host}, f)

	require.NoError(t, err)

	var agents []oac.Image
	require.NoError(t, json.NewDecoder(&buf).Decode(&agents))
	assert.GreaterOrEqual(t, len(agents), 1)
}

func TestRunDiscover_TableOutput(t *testing.T) {
	t.Parallel()

	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "test-agent",
	})
	push(t, img, host+"/test-agent:latest", craneOpts)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := commonFlags{outputJSON: false, noCache: true, rateLimit: 10, concurrency: 2}
	err := runDiscover(cmd, []string{host}, f)

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
