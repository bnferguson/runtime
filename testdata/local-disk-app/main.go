package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// GET /data returns the contents of /data/value (or 404 if not written yet)
	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			data, err := os.ReadFile("/data/value")
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Write(data)
		case http.MethodPost:
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			if err := os.WriteFile("/data/value", buf[:n], 0644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK\n")
	})

	fmt.Printf("Server starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
