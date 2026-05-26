package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	adminUser := os.Getenv("RUNNER_ADMIN_USER")
	adminPass := os.Getenv("RUNNER_ADMIN_PASS")
	if adminUser != "" && adminPass != "" {
		if err := ensureAdminUser(adminUser, adminPass); err != nil {
			log.Fatalf("Failed to seed admin user: %v", err)
		}
		log.Printf("Admin user '%s' seeded successfully.", adminUser)
	} else {
		log.Printf("Warning: RUNNER_ADMIN_USER or RUNNER_ADMIN_PASS not set. No default user created.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	mux := http.NewServeMux()
	
	// Public routes
	mux.HandleFunc("/api/v1/auth/login", handleLogin)
	mux.HandleFunc("/api/v1/auth/refresh", handleRefresh)
	
	// Protected routes
	mux.HandleFunc("/api/v1/run", withAuth(handleRun))
	mux.HandleFunc("/api/v1/upload", withAuth(handleUpload))
	mux.HandleFunc("/api/v1/edit-image", withAuth(handleEditImage))
	
	// Download route doesn't necessarily need Auth if you want it to be public for embedding,
	// but let's protect it so only authenticated users can download local files.
	// Actually, if the frontend embeds `<img src="...">`, it can't easily set Authorization headers!
	// So we should NOT protect the download endpoint with JWT, or we use a signed URL/query param.
	// For simplicity, let's make it public but it checks filepath.IsAbs().
	mux.HandleFunc("/api/v1/download", handleDownload)

	log.Printf("Agy Runner listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// withCORS is a simple middleware to add CORS headers
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
