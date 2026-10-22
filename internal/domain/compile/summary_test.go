package compile

import "testing"

func TestSizeMB(t *testing.T) {
	for in, want := range map[string]int{"35%": 11468, ".25": 8192, "8GiB": 8192, "2048M": 2048, "20G": 20480, "512MiB": 512, "huge": 0, "": 0} {
		if got := sizeMB(in, 32768); got != want {
			t.Errorf("sizeMB(%q) = %d, want %d", in, got, want)
		}
	}
}
