package run

import (
	"reflect"
	"testing"
)

func TestParseHardware(t *testing.T) {
	cases := map[string][]MachineGroup{
		"2x4c16g+1x2c4g": {{Count: 2, CPU: 4, MemoryGB: 16}, {Count: 1, CPU: 2, MemoryGB: 4}},
		"1x8c32g":        {{Count: 1, CPU: 8, MemoryGB: 32}},
		"":               {},
	}
	for key, want := range cases {
		if got := ParseHardware(key); !reflect.DeepEqual(got, want) {
			t.Errorf("ParseHardware(%q) = %v, want %v", key, got, want)
		}
	}
}
