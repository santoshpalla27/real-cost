package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/usage-engine/service"
)

func main() {
	http.HandleFunc("/forecast", handleForecast)
	http.HandleFunc("/health", handleHealth)

	port := ":8083"
	log.Printf("Usage Engine starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleForecast(w http.ResponseWriter, r *http.Request) {
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

	var req api.UsageForecastRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request JSON", http.StatusBadRequest)
		return
	}

	// Execute implementation
	forecast := service.Predict(req)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forecast); err != nil {
		log.Printf("Encode error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
