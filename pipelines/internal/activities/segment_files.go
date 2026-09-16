package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// writeSegmentFiles streams artifacts directly to the runner workspace. The
// injected reader lets tests exercise the actual filesystem path without a server.
func writeSegmentFiles(ctx context.Context, dir string, files []spec.SegmentFile, locations map[string]string, fetch func(context.Context, string) (io.ReadCloser, error)) error {
	if err := stroppycfg.ValidateFiles(files); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range files {
		if err := root.MkdirAll(filepath.Dir(f.Name), 0o755); err != nil {
			return err
		}
		if f.Ref == "" {
			if err := root.WriteFile(f.Name, []byte(f.Content), 0o644); err != nil {
				return err
			}
			continue
		}
		location := locations[f.Name]
		if location == "" {
			return fmt.Errorf("segment file %s: artifact was not resolved", f.Name)
		}
		src, err := fetch(ctx, location)
		if err != nil {
			return fmt.Errorf("segment file %s: %w", f.Name, err)
		}
		dst, err := root.OpenFile(f.Name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return fmt.Errorf("segment file %s: %w", f.Name, copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
