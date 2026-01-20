package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/graph"
)

// Config holds service URLs
type Config struct {
	IngestionURL  string
	SemanticURL   string
	EstimationURL string
	PolicyURL     string
}

// Defaults for local docker/dev
var cfg = Config{
	IngestionURL:  "http://localhost:8081/ingest",
	SemanticURL:   "http://localhost:8082/semantify",
	EstimationURL: "http://localhost:8085/estimate",
	PolicyURL:     "http://localhost:8086/evaluate",
}

func main() {
	planPath := flag.String("plan", "tfplan.json", "Path to terraform plan JSON")
	outputFormat := flag.String("output", "text", "Output format (text, json)")
	flag.Parse()

	// 1. Read Plan
	planJSON, err := os.ReadFile(*planPath)
	if err != nil {
		log.Fatalf("Failed to read plan file: %v", err)
	}

	// 2. Ingest
	fmt.Println("Analyzing infrastructure...")
	g, err := callIngestion(planJSON)
	if err != nil {
		log.Fatalf("Ingestion failed: %v", err)
	}

	// 3. Semantify
	fmt.Println("Mapping to billing components...")
	comps, err := callSemantic(g)
	if err != nil {
		log.Fatalf("Semantic mapping failed: %v", err)
	}

	// 4. Estimate
	fmt.Println("Predicting usage and estimating cost...")
	est, err := callEstimation(comps)
	if err != nil {
		log.Fatalf("Estimation failed: %v", err)
	}

	// 5. Policy Check
	fmt.Println("Verifying governance policies...")
	pRes, err := callPolicy(est)
	if err != nil {
		log.Fatalf("Policy check failed: %v", err)
	}

	// 6. Output
	if *outputFormat == "json" {
		out := struct {
			Estimate *api.EstimationResult `json:"estimate"`
			Policy   *api.PolicyResult     `json:"policy"`
		}{est, pRes}
		je := json.NewEncoder(os.Stdout)
		je.SetIndent("", "  ")
		je.Encode(out)
	} else {
		printTextReport(est, pRes)
	}
	
	if !pRes.Allowed {
		os.Exit(1)
	}
}

func callIngestion(plan []byte) (*graph.InfrastructureGraph, error) {
	resp, err := http.Post(cfg.IngestionURL, "application/json", bytes.NewBuffer(plan))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var g graph.InfrastructureGraph
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, err
	}
	return &g, nil
}

func callSemantic(g *graph.InfrastructureGraph) ([]api.BillingComponent, error) {
	body, _ := json.Marshal(g)
	resp, err := http.Post(cfg.SemanticURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var comps []api.BillingComponent
	if err := json.NewDecoder(resp.Body).Decode(&comps); err != nil {
		return nil, err
	}
	return comps, nil
}

func callEstimation(comps []api.BillingComponent) (*api.EstimationResult, error) {
	body, _ := json.Marshal(comps)
	resp, err := http.Post(cfg.EstimationURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	var est api.EstimationResult
	if err := json.NewDecoder(resp.Body).Decode(&est); err != nil {
		return nil, err
	}
	return &est, nil
}

func callPolicy(est *api.EstimationResult) (*api.PolicyResult, error) {
	body, _ := json.Marshal(est)
	resp, err := http.Post(cfg.PolicyURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var res api.PolicyResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func printTextReport(est *api.EstimationResult, p *api.PolicyResult) {
	fmt.Println("\n--- Cost Intelligence Report ---")
	fmt.Printf("Status:                   %s\n", getStatus(est))
	fmt.Printf("Confidence Score:         %.2f%%\n", est.ConfidenceScore*100)
	fmt.Printf("Total Monthly Cost (P50): $%.2f\n", est.TotalMonthlyCost.P50)
	fmt.Printf("Total Monthly Cost (P90): $%.2f\n", est.TotalMonthlyCost.P90)
	fmt.Printf("Carbon Footprint:         %.2f kgCO2e\n", est.Carbon.KgCO2e)
	
	if len(est.Errors) > 0 {
		fmt.Println("\n CRITICAL ERRORS:")
		for _, e := range est.Errors {
			fmt.Printf("  ! %s\n", e)
		}
	} else {
		fmt.Println("\n Drivers:")
		for _, d := range est.Drivers {
			fmt.Printf(" - %s: $%.2f (P90) [%s]\n", d.Component, d.P90Cost, d.Reason)
		}
	}

	fmt.Println("\n--- Governance ---")
	if p.Allowed {
		fmt.Println("✅ Plan Allowed")
	} else {
		fmt.Println("❌ Plan Rejected")
		for _, v := range p.Violations {
			fmt.Printf("  - VIOLATION: %s\n", v)
		}
	}
	for _, w := range p.Warnings {
		fmt.Printf("  - WARNING: %s\n", w)
	}
}

func getStatus(est *api.EstimationResult) string {
	if est.IsIncomplete {
		return "⚠️ INCOMPLETE"
	}
	if est.ConfidenceScore < 0.8 {
		return "⚠️ LOW CONFIDENCE"
	}
	return "✅ OK"
}
