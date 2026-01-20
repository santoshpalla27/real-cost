package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/futuristic-iac/ingestion-service/parser"
)

func main() {
	http.HandleFunc("/ingest", handleIngest)
	http.HandleFunc("/health", handleHealth)

	port := ":8081"
	log.Printf("IaC Ingestion Service starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	g, err := parser.Parse(body)
	if err != nil {
		log.Printf("Parse error: %v", err)
		http.Error(w, fmt.Sprintf("Parse error: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(g); err != nil {
		log.Printf("Encode error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
