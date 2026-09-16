package activities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthCommand(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		command, shell, want []string
		invalid              bool
	}{
		{name: "catalog postgres", command: []string{"CMD-SHELL", "pg_isready -h 127.0.0.1 -U postgres"}, want: []string{"/bin/sh", "-c", "pg_isready -h 127.0.0.1 -U postgres"}},
		{name: "exec form", command: []string{"CMD", "check", "literal $VAR"}, want: []string{"check", "literal $VAR"}},
		{name: "raw executable", command: []string{"pg_isready", "-U", "postgres"}, want: []string{"pg_isready", "-U", "postgres"}},
		{name: "image shell", command: []string{"CMD-SHELL", "test -f /ready"}, shell: []string{"/bin/bash", "-ec"}, want: []string{"/bin/bash", "-ec", "test -f /ready"}},
		{name: "empty", invalid: true},
		{name: "missing exec", command: []string{"CMD"}, invalid: true},
		{name: "missing script", command: []string{"CMD-SHELL"}, invalid: true},
		{name: "extra script args", command: []string{"CMD-SHELL", "true", "ignored"}, invalid: true},
		{name: "disabled", command: []string{"NONE"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := healthCommand(tc.command, tc.shell)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
