package spec

import "strings"

// MirrorImage routes an image of a public registry (Docker Hub, GHCR,
// Quay) through a registry mirror that proxies them as one group.
// Private and unsupported registries keep their explicit reference; an
// empty mirror leaves every image as is.
func MirrorImage(image, mirror string) string {
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

// MirrorImages returns the run with every container and the stroppy
// image routed through the mirror (MirrorImage).
func MirrorImages(r Run, mirror string) Run {
	if mirror == "" {
		return r
	}
	containers := make([]Container, len(r.Containers))
	for i, c := range r.Containers {
		c.Image = MirrorImage(c.Image, mirror)
		containers[i] = c
	}
	r.Containers = containers
	r.Workload.StroppyImage = MirrorImage(r.Workload.StroppyImage, mirror)
	return r
}
