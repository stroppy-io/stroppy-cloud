package main

import "testing"

func TestMirrorImage(t *testing.T) {
	for _, tc := range []struct{ image, mirror, want string }{
		{"postgres:17", "docker.stroppy.io", "docker.stroppy.io/library/postgres:17"},
		{"docker.io/postgres:17", "docker.stroppy.io", "docker.stroppy.io/library/postgres:17"},
		{"docker.io/library/postgres:17", "docker.stroppy.io", "docker.stroppy.io/library/postgres:17"},
		{"quay.io/prometheus/node-exporter:v1.12.1", "docker.stroppy.io", "docker.stroppy.io/prometheus/node-exporter:v1.12.1"},
		{"ghcr.io/stroppy-io/stroppy@sha256:abcd", "mirror:5000", "mirror:5000/stroppy-io/stroppy@sha256:abcd"},
		{"bitnami/etcd:3", "docker.stroppy.io", "docker.stroppy.io/bitnami/etcd:3"},
		{"cr.yandex/private/image:v1", "docker.stroppy.io", "cr.yandex/private/image:v1"},
		{"docker.stroppy.io/library/postgres:17", "docker.stroppy.io", "docker.stroppy.io/library/postgres:17"},
		{"postgres:17", "", "postgres:17"},
	} {
		if got := mirrorImage(tc.image, tc.mirror); got != tc.want {
			t.Errorf("mirrorImage(%q, %q) = %q; want %q", tc.image, tc.mirror, got, tc.want)
		}
	}
}
