package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/futuristic-iac/semantic-engine/mapper"
	// Register mappers
	_ "github.com/futuristic-iac/semantic-engine/mapper/aws"
	"github.com/futuristic-iac/pkg/graph"
	"github.com/futuristic-iac/pkg/api"
)

func main() {
	http.HandleFunc("/semantify", handleSemantify)
	http.HandleFunc("/health", handleHealth)

	port := ":8082"
	log.Printf("Semantic Engine starting on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleSemantify(w http.ResponseWriter, r *http.Request) {
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

	var g graph.InfrastructureGraph
	if err := json.Unmarshal(body, &g); err != nil {
		http.Error(w, "Invalid graph JSON", http.StatusBadRequest)
		return
	}

	var allComponents []api.BillingComponent

	for _, res := range g.Resources {
		comps, err := mapper.MapResource(res)
		if err != nil {
			log.Printf("Mapping error for %s: %v", res.Address, err)
			continue
		}
		if comps != nil {
			allComponents = append(allComponents, comps...)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(allComponents); err != nil {
		log.Printf("Encode error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
