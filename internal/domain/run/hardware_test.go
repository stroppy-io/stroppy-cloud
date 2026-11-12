package run

import (
	"reflect"
	"testing"
)

func TestParseHardware(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		key  string
		want []MachineGroup
	}{
		{"2x4c16g:standard-v3:network-ssd+1x2c4g:standard-v3:", []MachineGroup{
			{Count: 2, CPU: 4, MemoryGB: 16, Platform: "standard-v3", DiskType: "network-ssd"},
			{Count: 1, CPU: 2, MemoryGB: 4, Platform: "standard-v3"},
		}},
		{"1x32c128g:highfreq-v3:network-ssd-nonreplicated", []MachineGroup{
			{Count: 1, CPU: 32, MemoryGB: 128, Platform: "highfreq-v3", DiskType: "network-ssd-nonreplicated"},
		}},
		// A snapshot without the platform and disk fields: the shape alone.
		{"1x8c32g::", []MachineGroup{{Count: 1, CPU: 8, MemoryGB: 32}}},
		{"", []MachineGroup{}},
		{"garbage", []MachineGroup{}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			if got := ParseHardware(tc.key); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseHardware(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func BenchmarkParseHardware(b *testing.B) {
	for b.Loop() {
		ParseHardware("3x32c128g:standard-v3:network-ssd-nonreplicated+1x4c8g:standard-v3:network-ssd")
	}
}
