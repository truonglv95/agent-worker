package main

import (
	"net/http"
	"os"
	"path/filepath"
)

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	// Basic check to ensure path is absolute (in reality, more security checks needed)
	if !filepath.IsAbs(path) {
		http.Error(w, "Path must be absolute", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, path)
}
