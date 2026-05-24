package oac

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors returned by Parse and Validate.
var (
	ErrUnsupportedVersion   = errors.New("unsupported spec version")
	ErrNoSpec               = errors.New("no spec populated for version")
	ErrNameRequired         = errors.New("name is required")
	ErrOrchestratorRequired = errors.New("orchestrator is required")
	ErrSessionIsolation     = errors.New("session.isolation cannot be combined with workspaces")
)

// Parse parses OAC labels into a Manifest, returning an error for unknown
// versions or unrecognised OAC-prefixed labels.
func Parse(labels map[string]string) (*Manifest, error) {
	version := labels[LabelVersion]
	m := &Manifest{Version: version}

	var err error

	switch version {
	case VersionV1Alpha1:
		m.V1Alpha1, err = parseV1Alpha1(labels)
	case VersionV1Alpha2:
		m.V1Alpha2, err = parseV1Alpha2(labels)
	default:
		err = fmt.Errorf("%w %q", ErrUnsupportedVersion, version)
	}

	if err != nil {
		return nil, err
	}

	return m, nil
}

func parseV1Alpha1(labels map[string]string) (*V1Alpha1Spec, error) {
	data, err := json.Marshal(labelsToTree(labels))
	if err != nil {
		return nil, err
	}

	var spec V1Alpha1Spec

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	err = dec.Decode(&spec)
	if err != nil {
		return nil, err
	}

	return &spec, nil
}

func parseV1Alpha2(labels map[string]string) (*V1Alpha2Spec, error) {
	data, err := json.Marshal(labelsToTree(labels))
	if err != nil {
		return nil, err
	}

	var spec V1Alpha2Spec

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	err = dec.Decode(&spec)
	if err != nil {
		return nil, err
	}

	return &spec, nil
}

// labelsToTree strips labelPrefix from each key, skips "version",
// splits remaining suffix on ".", and builds a nested map[string]any.
// Leaf values: "true"/"false" → bool, everything else → string.
func labelsToTree(labels map[string]string) map[string]any {
	tree := make(map[string]any)

	for key, val := range labels {
		if !strings.HasPrefix(key, labelPrefix) {
			continue
		}

		suffix := strings.TrimPrefix(key, labelPrefix)

		if suffix == "version" {
			continue
		}

		parts := strings.Split(suffix, ".")

		setNestedValue(tree, parts, coerceLabelValue(val))
	}

	return tree
}

func setNestedValue(m map[string]any, parts []string, val any) {
	if len(parts) == 1 {
		m[parts[0]] = val

		return
	}

	key := parts[0]

	sub, ok := m[key].(map[string]any)

	if !ok {
		sub = make(map[string]any)
		m[key] = sub
	}

	setNestedValue(sub, parts[1:], val)
}

func coerceLabelValue(val string) any {
	switch val {
	case "true":
		return true
	case "false":
		return false
	default:
		return val
	}
}
