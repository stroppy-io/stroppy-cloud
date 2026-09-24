// Package schemajson writes a schema as JSON one way, byte for byte.
//
// protojson deliberately varies its whitespace between runs so nobody
// depends on it; a generated file in the repository does depend on it —
// it is committed, reviewed and checked by CI. So the protoJSON is
// re-encoded through encoding/json, which sorts object keys and spaces
// them the same way every time, with number literals kept verbatim.
package schemajson

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Marshal returns the canonical protoJSON of m, indented and newline
// terminated, ready to be written to a generated file.
func Marshal(m proto.Message) ([]byte, error) {
	raw, err := protojson.Marshal(m)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Keep 1e+06 and 1048576 as protojson wrote them: a float64 round
	// trip would rewrite both.
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("schemajson: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("schemajson: %w", err)
	}
	return append(out, '\n'), nil
}
