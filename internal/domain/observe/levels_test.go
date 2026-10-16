package observe

import (
	"slices"
	"testing"
)

func TestLogLevelNamesOnlyKnownLevels(t *testing.T) {
	for in, want := range map[string]string{
		"INFO": "info", "info2": "info", "WARNING": "warn", "Warn": "warn", "ERROR": "error", "err": "error",
		"FATAL4": "fatal", "critical": "fatal", "TRACE": "trace", "debug3": "debug", "UNSPECIFIED": "", "": "", "W>": "",
	} {
		got := LogLevel(in)
		if got != want {
			t.Errorf("LogLevel(%q) = %q, want %q", in, got, want)
		}
		if got != "" && !slices.Contains(LogLevels, got) {
			t.Errorf("LogLevel(%q) = %q is not in LogLevels", in, got)
		}
	}
}
