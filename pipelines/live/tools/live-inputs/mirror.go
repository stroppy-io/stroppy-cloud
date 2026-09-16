package main

import "strings"

// mirrorImage routes only the public registries configured in the live Nexus
// group. Private and unsupported registries keep their explicit reference.
func mirrorImage(image, mirror string) string {
	if mirror == "" {
		return image
	}
	first, rest, slash := strings.Cut(image, "/")
	repository := image
	if slash && (strings.ContainsAny(first, ".:") || first == "localhost") {
		switch first {
		case "docker.io", "index.docker.io", "registry-1.docker.io", "ghcr.io", "quay.io":
			repository = rest
		default:
			return image
		}
	}
	if !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}
	return mirror + "/" + repository
}
