package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// uploadDir is where uploaded packages are stored. Served at /packages/
const uploadDir = "/tmp/stroppy-packages"

func (s *Server) uploadDeb(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 500MB)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, "failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate filename extension
	if ext := filepath.Ext(header.Filename); ext != ".deb" {
		http.Error(w, "only .deb files are accepted, got "+ext, http.StatusBadRequest)
		return
	}

	// Save to uploadDir
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "create upload dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	destPath := filepath.Join(uploadDir, filepath.Base(header.Filename))
	dest, err := os.Create(destPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		http.Error(w, "write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the URL where agents can download it
	url := fmt.Sprintf("http://%s/packages/%s", r.Host, filepath.Base(header.Filename))
	writeJSON(w, http.StatusOK, map[string]string{
		"filename": header.Filename,
		"url":      url,
		"size":     fmt.Sprintf("%d", written),
	})
}
