package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDVal := r.Context().Value("userID")
	if userIDVal == nil {
		http.Error(w, "Unauthorized context", http.StatusUnauthorized)
		return
	}
	userID := userIDVal.(int)

	// Max 10 MB per file
	r.ParseMultipartForm(10 << 20)
	
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	userDir := filepath.Join(os.TempDir(), "runner_uploads", fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userDir, 0755); err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	// Sanitize filename and construct path
	filename := filepath.Base(handler.Filename)
	dstPath := filepath.Join(userDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Failed to save the file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to write the file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Upload successful",
		"path":    dstPath,
	})
}
