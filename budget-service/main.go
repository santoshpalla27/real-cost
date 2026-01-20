package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/budget", handleGetBudget)

	port := ":8087"
	log.Printf("Budget API (Mock) starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type BudgetResponse struct {
	TotalBudget float64  `json:"total_budget"`
	Currency    string   `json:"currency"`
	Tags        []string `json:"tags"`
}

func handleGetBudget(w http.ResponseWriter, r *http.Request) {
	// Mock: Always return $1200 for now.
	// In reality, this would query a DB based on Project ID.
	resp := BudgetResponse{
		TotalBudget: 1200.0,
		Currency:    "USD",
		Tags:        []string{"prod", "aws"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
