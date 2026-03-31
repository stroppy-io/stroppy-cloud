package web

import "embed"

// Dist contains the built SPA files from web/dist/.
// Build with: cd web && npx vite build
//
//go:embed dist/*
var Dist embed.FS
