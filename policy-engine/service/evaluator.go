package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/futuristic-iac/pkg/api"
)

type Evaluator struct {
	OpaURL string
	Auditor *AuditLogger
}

func NewEvaluator(opaURL string) *Evaluator {
	return &Evaluator{
		OpaURL: opaURL,
		Auditor: NewAuditLogger("policy_audit.log"),
	}
}

func (e *Evaluator) Evaluate(estimate *api.EstimationResult) (*api.PolicyResult, error) {
	// 1. Fetch Budget (Mock: normally http.Get("http://budget-service..."))
	budget := map[string]interface{}{
		"total_budget": 1200.0,
	}

	req := map[string]interface{}{
		"input": map[string]interface{}{
			"estimate": estimate,
			"budget":   budget,
		},
	}
	// Note: We are using a map here because PolicyRequest struct in pkg/api might be shared/strict.
	// Actually better to update pkg/api definitions if we want type safety. 
	// But let's use the struct defined in pkg/api if I updated it correctly.
	// I updated pkg/api/policy.go in the previous step? No, I tried to update it in THIS step but I can't target two files.
	// I will update pkg/api/policy.go separately.
	// Here I will assume pkg/api is updated.

	body, _ := json.Marshal(req)
	// Querying the 'cost/allow' rule
	resp, err := http.Post(e.OpaURL+"/v1/data/cost/governance", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OPA status: %d", resp.StatusCode)
	}

	// OPA response format: { "result": { "allow": true, "violations": [...] } }
	var opaResp struct {
		Result struct {
			Allow      bool     `json:"allow"`
			Violations []string `json:"violations"`
			Warnings   []string `json:"warnings"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&opaResp); err != nil {
		return nil, err
	}

	res := &api.PolicyResult{
		Allowed:    opaResp.Result.Allow,
		Violations: opaResp.Result.Violations,
		Warnings:   opaResp.Result.Warnings,
	}
	
	// Audit Log
	if err := e.Auditor.Log(estimate, res); err != nil {
		fmt.Printf("Audit log failed: %v\n", err)
	}

	return res, nil
}
