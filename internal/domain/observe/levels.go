package observe

import "strings"

// LogLevels is every level a log line carries (the contract's LogLevel).
var LogLevels = []string{"trace", "debug", "info", "warn", "error", "fatal"}

// LogLevel names a source severity by LogLevels: OTLP severity texts and
// the usual spellings ("WARNING", "Err", "INFO2", "critical"); empty for
// none (OTLP UNSPECIFIED) or a text that names no level.
func LogLevel(severity string) string {
	l := strings.ToLower(strings.TrimSpace(severity))
	l = strings.TrimRight(l, "0123456789")
	switch l {
	case "trace":
		return "trace"
	case "debug":
		return "debug"
	case "info", "information", "notice":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error", "err":
		return "error"
	case "fatal", "critical", "crit", "panic", "emerg", "emergency", "alert":
		return "fatal"
	}
	return ""
}
