package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// UnmarshalJSON keeps unknown top-level/nested typed fields from disappearing
// before workflow validation, and preserves absent versus explicit false.
func (r *Run) UnmarshalJSON(data []byte) error {
	type plain Run
	out := plain{Network: Network{AllowPublicIPs: true}}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return err
	}
	*r = Run(out)
	return nil
}

// UnmarshalJSON uses the same default as spec.suite@1 for omitted settings.
func (s *Suite) UnmarshalJSON(data []byte) error {
	type plain Suite
	out := plain{Defaults: SuiteDefaults{ContinueOnFailure: true}}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return err
	}
	*s = Suite(out)
	return nil
}

// DecodeObject preserves integer precision in dynamic schema input (notably
// uint64 seeds), while returning the concrete numeric types schemapb accepts.
func DecodeObject(data []byte) (map[string]any, error) {
	var value map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	out, ok := exactNumbers(value).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("input must be an object")
	}
	return out, nil
}

func exactNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
		if n, err := strconv.ParseUint(string(v), 10, 64); err == nil {
			return n
		}
		if n, err := v.Float64(); err == nil {
			return n
		}
		return string(v)
	case map[string]any:
		for key, x := range v {
			v[key] = exactNumbers(x)
		}
	case []any:
		for i, x := range v {
			v[i] = exactNumbers(x)
		}
	}
	return value
}
