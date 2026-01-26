package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: dns-test <web|backend>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "web":
		runWeb()
	case "backend":
		runBackend()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBackend() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from backend!")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	fmt.Printf("Backend starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting backend: %v\n", err)
		os.Exit(1)
	}
}

func runWeb() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// The key test: can we resolve backend.app.miren?
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "DNS Test App\n")
		fmt.Fprintf(w, "============\n\n")
		fmt.Fprintf(w, "Use /dns-lookup to test DNS resolution\n")
		fmt.Fprintf(w, "Use /call-backend to test HTTP call to backend\n")
	})

	http.HandleFunc("/dns-lookup", func(w http.ResponseWriter, r *http.Request) {
		host := "backend.app.miren"
		fmt.Fprintf(w, "DNS Lookup Test\n")
		fmt.Fprintf(w, "===============\n\n")
		fmt.Fprintf(w, "Looking up: %s\n\n", host)

		ips, err := net.LookupHost(host)
		if err != nil {
			fmt.Fprintf(w, "ERROR: %v\n", err)
			fmt.Fprintf(w, "\nThis is the MIR-634 bug - DNS watcher not started!\n")
			return
		}

		fmt.Fprintf(w, "SUCCESS! Resolved to:\n")
		for _, ip := range ips {
			fmt.Fprintf(w, "  - %s\n", ip)
		}
	})

	http.HandleFunc("/call-backend", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Backend Call Test\n")
		fmt.Fprintf(w, "=================\n\n")

		// Try to call backend.app.miren
		url := "http://backend.app.miren:8080/"
		fmt.Fprintf(w, "Calling: %s\n\n", url)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Fprintf(w, "ERROR: %v\n", err)
			fmt.Fprintf(w, "\nThis might be the MIR-634 bug - DNS watcher not started!\n")
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(w, "ERROR reading response: %v\n", err)
			return
		}

		fmt.Fprintf(w, "SUCCESS! Response from backend:\n")
		fmt.Fprintf(w, "Status: %d\n", resp.StatusCode)
		fmt.Fprintf(w, "Body: %s\n", string(body))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	fmt.Printf("Web starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting web: %v\n", err)
		os.Exit(1)
	}
}
