package spec

import (
	"errors"
	"slices"
	"strings"
)

// ValidationError preserves findings across compiler and service boundaries.
// Issues contain paths and diagnostics, never rejected config/credential values.
type ValidationError struct {
	Issues []ResourceIssue
}

func (e *ValidationError) Error() string {
	var messages []string
	for _, i := range e.Issues {
		if i.Severity == "ERROR" {
			messages = append(messages, i.Path+": "+i.Message)
		}
	}
	return "invalid pipeline input: " + strings.Join(messages, "; ")
}

// WithValidationPath maps an override-local path to its public input document.
func WithValidationPath(err error, prefix string) error {
	var v *ValidationError
	if !errors.As(err, &v) {
		return err
	}
	issues := slices.Clone(v.Issues)
	for i := range issues {
		issues[i].Path = strings.TrimSuffix(prefix+"."+issues[i].Path, ".")
		issues[i].Scope = "input"
	}
	return &ValidationError{Issues: issues}
}
