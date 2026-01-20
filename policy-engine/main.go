package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/policy-engine/service"
)

func main() {
	evaluator := service.NewEvaluator("http://localhost:8181")

	http.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
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

		var estimate api.EstimationResult
		if err := json.Unmarshal(body, &estimate); err != nil {
			http.Error(w, "Invalid input JSON", http.StatusBadRequest)
			return
		}

		result, err := evaluator.Evaluate(&estimate)
		if err != nil {
			log.Printf("Evaluation error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := ":8086"
	log.Printf("Policy Engine starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
