package activities

import "fmt"

// Docker healthcheck markers are configuration syntax, not executable names.
func healthCommand(command, shell []string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty healthcheck command")
	}
	switch command[0] {
	case "CMD":
		if len(command) < 2 {
			return nil, fmt.Errorf("CMD healthcheck needs an executable")
		}
		return command[1:], nil
	case "CMD-SHELL":
		if len(command) != 2 {
			return nil, fmt.Errorf("CMD-SHELL healthcheck needs exactly one script")
		}
		if len(shell) == 0 {
			shell = []string{"/bin/sh", "-c"}
		}
		return append(append([]string{}, shell...), command[1]), nil
	case "NONE":
		return nil, fmt.Errorf("omit the healthcheck to disable it")
	default:
		return command, nil
	}
}
