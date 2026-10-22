package activities

import (
	"errors"
	"testing"
)

func TestTransientRegistryError(t *testing.T) {
	for msg, want := range map[string]bool{
		`failed to do request: Get "https://docker.stroppy.io/v2/coreos/etcd/manifests/sha256:58d7": net/http: timeout awaiting response headers`: true,
		"received unexpected HTTP status: 504 Gateway Timeout": true,
		"dial tcp 10.0.0.1:443: i/o timeout":                   true,
		"manifest unknown: manifest unknown":                   false,
		"unauthorized: authentication required":                false,
	} {
		if got := transientRegistryError(errors.New(msg)); got != want {
			t.Errorf("%q transient = %v, want %v", msg, got, want)
		}
	}
}
