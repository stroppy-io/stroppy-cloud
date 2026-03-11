package migrations

import "embed"

//go:embed *.sql atlas.sum
var Content embed.FS
