package yandex

import (
	"embed"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

//go:embed *.tf
var tfFilesEmbed embed.FS

// EmbeddedTfFiles returns all .tf files from the embedded filesystem as TfFile slices.
func EmbeddedTfFiles() ([]terraform.TfFile, error) {
	entries, err := tfFilesEmbed.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var files []terraform.TfFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := tfFilesEmbed.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, terraform.NewTfFile(content, e.Name()))
	}
	return files, nil
}
