package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/futuristic-iac/pkg/api"
)

// Policy Regression Suite
// Tries to send various payloads to the Policy Engine and asserts the outcome.

func main() {
	fmt.Println("Running Policy Regression Tests...")
	
	tests := []struct {
		Name          string
		Estimate      api.EstimationResult
		ExpectedAllow bool
	}{
		{
			Name: "High Confidence, Low Cost -> ALLOW",
			Estimate: api.EstimationResult{
				TotalMonthlyCost: struct{ P50, P90 float64 `json:"p50";json:"p90"` }{500, 500},
				ConfidenceScore: 0.95,
				IsIncomplete:    false,
			},
			ExpectedAllow: true,
		},
		{
			Name: "High Confidence, High Cost -> DENY",
			Estimate: api.EstimationResult{
				TotalMonthlyCost: struct{ P50, P90 float64 `json:"p50";json:"p90"` }{1500, 1500},
				ConfidenceScore: 0.95,
				IsIncomplete:    false,
			},
			ExpectedAllow: false,
		},
		{
			Name: "Low Confidence -> DENY",
			Estimate: api.EstimationResult{
				TotalMonthlyCost: struct{ P50, P90 float64 `json:"p50";json:"p90"` }{100, 100},
				ConfidenceScore: 0.5,
				IsIncomplete:    false,
			},
			ExpectedAllow: false,
		},
		{
			Name: "Incomplete -> DENY",
			Estimate: api.EstimationResult{
				TotalMonthlyCost: struct{ P50, P90 float64 `json:"p50";json:"p90"` }{100, 100},
				ConfidenceScore: 1.0,
				IsIncomplete:    true,
				Errors:          []string{"Missing Price"},
			},
			ExpectedAllow: false,
		},
	}
	
	pass := 0
	fail := 0
	
	for _, t := range tests {
		fmt.Printf("TEST: %s ... ", t.Name)
		
		// Note: The Evaluator in the service mocks the Budget fetch. 
		// So we are testing the service Integration + OPA.
		
		body, _ := json.Marshal(t.Estimate)
		resp, err := http.Post("http://localhost:8086/evaluate", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Printf("ERROR calling service: %v\n", err)
			fail++
			continue
		}
		defer resp.Body.Close()
		
		var res api.PolicyResult
		json.NewDecoder(resp.Body).Decode(&res)
		
		if res.Allowed == t.ExpectedAllow {
			fmt.Println("PASS")
			pass++
		} else {
			fmt.Printf("FAIL (Got %v, Expected %v)\n", res.Allowed, t.ExpectedAllow)
			fail++
		}
	}
	
	fmt.Printf("\nResult: %d PASS, %d FAIL\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}
