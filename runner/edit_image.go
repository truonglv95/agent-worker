package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type EditImageRequest struct {
	ImagePath string `json:"image_path"`
	Prompt    string `json:"prompt"`
	Stream    bool   `json:"stream"`
}

func handleEditImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EditImageRequest

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		req.Prompt = r.FormValue("prompt")
		req.ImagePath = r.FormValue("image_path")
		req.Stream = r.FormValue("stream") == "true"

		file, handler, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			userIDVal := r.Context().Value("userID")
			if userIDVal == nil {
				http.Error(w, "Unauthorized context", http.StatusUnauthorized)
				return
			}
			userID := userIDVal.(int)
			userDir := filepath.Join(os.TempDir(), "runner_uploads", fmt.Sprintf("user_%d", userID))
			os.MkdirAll(userDir, 0755)

			dstPath := filepath.Join(userDir, filepath.Base(handler.Filename))
			dst, err := os.Create(dstPath)
			if err != nil {
				http.Error(w, "Failed to save the image", http.StatusInternalServerError)
				return
			}
			io.Copy(dst, file)
			dst.Close()

			req.ImagePath = dstPath
		} else if err != http.ErrMissingFile {
			http.Error(w, "Error retrieving image file", http.StatusBadRequest)
			return
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	if req.ImagePath == "" || req.Prompt == "" {
		http.Error(w, "image_path and prompt are required", http.StatusBadRequest)
		return
	}

	// Verify image exists
	if _, err := os.Stat(req.ImagePath); os.IsNotExist(err) {
		http.Error(w, "Image path does not exist", http.StatusBadRequest)
		return
	}

	// Check if R2 is configured
	useR2 := false
	if os.Getenv("R2_ACCOUNT_ID") != "" && os.Getenv("R2_ACCESS_KEY_ID") != "" && os.Getenv("R2_BUCKET_NAME") != "" {
		useR2 = true
	}

	var instructions string
	cwd, _ := os.Getwd()
	if useR2 {
		scriptPath := filepath.Join(cwd, "upload_r2.sh")
		instructions = fmt.Sprintf(`You are an Image Editing Assistant.
The user wants you to edit the image located exactly at: %s
Instructions: %s
CRITICAL RULE: When you finish editing the image, you MUST upload it to Cloudflare R2 using the bash script at %s.
Run: bash %s <path_to_the_new_image>
The script will print out a public URL. You MUST return ONLY that public URL (or embed it as an image) to the user in your final response. Do NOT return the local absolute path.`,
			req.ImagePath, req.Prompt, scriptPath, scriptPath)
	} else {
		// Fallback to local path mapping
		// Assuming the client will prepend the tunnel domain themselves, or we instruct AI to format it.
		instructions = fmt.Sprintf(`You are an Image Editing Assistant.
The user wants you to edit the image located exactly at: %s
Instructions: %s
CRITICAL RULE: When you finish editing the image, you MUST return the absolute path to the newly generated image formatted EXACTLY like this:
/api/v1/download?path=<absolute_path_here>
Do not use markdown links, just print the path.`,
			req.ImagePath, req.Prompt)
	}

	args := []string{
		"--print", instructions,
		"--dangerously-skip-permissions",
		"--log-file", "/dev/stderr",
	}

	cmd := exec.Command("agy", args...)

	if req.Stream {
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			http.Error(w, "Failed to get stdout pipe", http.StatusInternalServerError)
			return
		}

		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		if err := cmd.Start(); err != nil {
			http.Error(w, "Failed to start command", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if isGlogLine(line) {
				continue
			}
			chunk, _ := json.Marshal(map[string]string{"text": line + "\n"})
			w.Write([]byte("data: " + string(chunk) + "\n\n"))
			flusher.Flush()
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("[runner] stream CLI error: %v, stderr: %s", err, stderrBuf.String())
			errChunk, _ := json.Marshal(map[string]string{"error": err.Error(), "stderr": stderrBuf.String()})
			w.Write([]byte("data: " + string(errChunk) + "\n\n"))
			flusher.Flush()
		}

		w.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
		return
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		log.Printf("[runner] CLI error: %v, stderr: %s", err, stderrBuf.String())
		http.Error(w, "CLI execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cleanOutput := filterGlogLines(stdoutBuf.String())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"output": cleanOutput,
	})
}
