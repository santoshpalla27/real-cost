package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/futuristic-iac/pkg/focus"
	"github.com/futuristic-iac/pricing-engine/storage"
)

var store storage.PricingStore

func main() {
	// Initialize Store
	store = storage.NewClickHouseStore()

	// Seed some data? 
	// In real life, correct ingestion runs separately.

	http.HandleFunc("/price", handleGetPrice)
	http.HandleFunc("/health", handleHealth)

	port := ":8084"
	log.Printf("Pricing Engine starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type PriceRequest struct {
	Provider   string            `json:"provider"`
	Attributes map[string]string `json:"attributes"`
	Timestamp  time.Time         `json:"timestamp"`
}

func handleGetPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	price, err := store.GetPrice(r.Context(), req.Provider, req.Attributes, req.Timestamp)
	if err != nil {
		// Differentiate between "Not Found" and "DB Error"
		// For MVP, assuming non-nil error from GetPrice implies not found if it was a lookup issue.
		// In production, check error type.
		log.Printf("Price lookup failed for %v: %v", req.Attributes, err)
		http.Error(w, fmt.Sprintf("Price not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}
