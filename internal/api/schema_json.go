package api

import (
	"bytes"
	"encoding/json"
	"math/big"
)

// browserSchemaJSON preserves exact large integers through JSON.parse/stringify.
// schemapb integer fields accept decimal strings; TS exports number | string.
// Native opaque reports do not go through this schema-value conversion.
func browserSchemaJSON(raw json.RawMessage) json.RawMessage {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if d.Decode(&value) != nil {
		return raw
	}
	var visit func(any) any
	visit = func(v any) any {
		switch x := v.(type) {
		case json.Number:
			if n, ok := new(big.Int).SetString(x.String(), 10); ok && n.Abs(n).Cmp(big.NewInt(9007199254740991)) > 0 {
				return x.String()
			}
		case map[string]any:
			for k, item := range x {
				x[k] = visit(item)
			}
		case []any:
			for i, item := range x {
				x[i] = visit(item)
			}
		}
		return v
	}
	out, err := json.Marshal(visit(value))
	if err != nil {
		return raw
	}
	return out
}
