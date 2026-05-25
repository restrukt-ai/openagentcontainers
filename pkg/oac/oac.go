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
	ErrUnsupportedVersion       = errors.New("unsupported spec version")
	ErrNoSpec                   = errors.New("no spec populated for version")
	ErrNameRequired             = errors.New("name is required")
	ErrOrchestratorRequired     = errors.New("orchestrator is required")
	ErrOrchestratorEnvRequired  = errors.New("orchestrator.env is required")
	ErrOrchestratorAuthRequired = errors.New("orchestrator must declare at least one auth method")
	ErrSessionIsolation         = errors.New("session.isolation cannot be combined with workspaces")
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

// decodeStrict decodes v into dst using a strict decoder that rejects unknown fields.
func decodeStrict(v json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.DisallowUnknownFields()

	return dec.Decode(dst)
}

// decodeEnvFileField decodes the named key from raw into dst (an *EnvFile),
// removes the key from raw, and reports whether the key was present.
// Returns an error only when the key is present but decoding fails.
func decodeEnvFileField(raw map[string]json.RawMessage, key string, dst **EnvFile) error {
	v, ok := raw[key]
	if !ok {
		return nil
	}

	ef := new(EnvFile)

	err := decodeStrict(v, ef)
	if err != nil {
		return err
	}

	*dst = ef

	delete(raw, key)

	return nil
}

// inferenceTypesFromRaw decodes every remaining entry in raw as an
// InferenceTypeSpec and returns the resulting map.
func inferenceTypesFromRaw(raw map[string]json.RawMessage) (map[string]InferenceTypeSpec, error) {
	var types map[string]InferenceTypeSpec

	for k, v := range raw {
		var ts InferenceTypeSpec

		err := decodeStrict(v, &ts)
		if err != nil {
			return nil, fmt.Errorf("inference.%s: %w", k, err)
		}

		if types == nil {
			types = make(map[string]InferenceTypeSpec, len(raw))
		}

		types[k] = ts
	}

	return types, nil
}

// UnmarshalJSON implements custom unmarshaling for InferenceSpec.
// Known keys "api_base" and "api_key" are decoded as *EnvFile fields.
// All remaining keys are treated as inference type names and decoded as
// InferenceTypeSpec values. Unknown sub-fields within a type spec are rejected.
func (s *InferenceSpec) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	err = decodeEnvFileField(raw, "api_base", &s.APIBase)
	if err != nil {
		return fmt.Errorf("inference.api_base: %w", err)
	}

	err = decodeEnvFileField(raw, "api_key", &s.APIKey)
	if err != nil {
		return fmt.Errorf("inference.api_key: %w", err)
	}

	s.Types, err = inferenceTypesFromRaw(raw)
	if err != nil {
		return err
	}

	return nil
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
