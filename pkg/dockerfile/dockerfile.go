// Package dockerfile parses Dockerfiles to extract OAC labels.
//
// Use [ParseLabels] to extract raw label key-value pairs from any [io.Reader].
// Use [Parse] to open a file by path and return a fully-decoded *oac.Dockerfile.
//
//	df, err := dockerfile.Parse("/path/to/Dockerfile")
//	if err != nil { ... }
//	fmt.Println(df.Name(), df.Path)
package dockerfile

import (
	"io"
	"os"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// ParseLabels reads r as a Dockerfile and returns all LABEL key-value pairs.
// It correctly handles line continuations, quoted values, case-insensitive LABEL
// keywords, and multiple key-value pairs on a single instruction.
func ParseLabels(r io.Reader) (map[string]string, error) {
	result, err := parser.Parse(r)
	if err != nil {
		return nil, err
	}

	labels := make(map[string]string)

	for _, node := range result.AST.Children {
		if !strings.EqualFold(node.Value, "label") {
			continue
		}
		// buildkit AST: key → "value" → = → key → "value" → = → ...
		// Quotes are not stripped by the parser; "=" nodes separate pairs.
		for tok := node.Next; tok != nil && tok.Next != nil; {
			key := tok.Value
			val := strings.TrimPrefix(strings.TrimSuffix(tok.Next.Value, `"`), `"`)
			labels[key] = val

			tok = tok.Next.Next
			if tok != nil && tok.Value == "=" {
				tok = tok.Next
			}
		}
	}

	return labels, nil
}

// Parse opens the Dockerfile at path, extracts its labels, and decodes them into
// an *oac.Dockerfile. Returns an error if the file cannot be read, the Dockerfile
// is syntactically invalid, or the OAC labels fail to parse (e.g. unknown version).
func Parse(path string) (*oac.Dockerfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	labels, err := ParseLabels(f)
	if err != nil {
		return nil, err
	}

	manifest, err := oac.Parse(labels)
	if err != nil {
		return nil, err
	}

	return &oac.Dockerfile{Manifest: *manifest, Path: path}, nil
}
