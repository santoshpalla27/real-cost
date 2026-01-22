// Package policy provides OPA policy evaluation.
package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/rego"
	"github.com/santoshpalla27/fiac-platform/internal/estimation"
)

// Result holds policy evaluation outcomes.
type Result struct {
	Denials  []string `json:"denials"`
	Warnings []string `json:"warnings"`
	Passed   bool     `json:"passed"`
}

// Evaluator runs OPA policies against estimation results.
type Evaluator struct {
	policiesDir string
}

func NewEvaluator(policiesDir string) *Evaluator {
	return &Evaluator{policiesDir: policiesDir}
}

func (e *Evaluator) Evaluate(est *estimation.Result) (*Result, error) {
	result := &Result{Denials: []string{}, Warnings: []string{}, Passed: true}

	input := map[string]any{
		"total_cost_p50":      est.TotalCost.P50,
		"total_cost_p90":      est.TotalCost.P90,
		"carbon_kg":           est.TotalCarbon.KgCO2e,
		"confidence_score":    est.ConfidenceScore,
		"is_incomplete":       est.IsIncomplete,
		"cost_growth_percent": 0.0,
	}

	files, err := filepath.Glob(filepath.Join(e.policiesDir, "*.rego"))
	if err != nil || len(files) == 0 {
		return result, nil
	}

	for _, file := range files {
		policy, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		denials, err := e.evalQuery(string(policy), "data.fiac.deny", input)
		if err == nil {
			result.Denials = append(result.Denials, denials...)
		}

		warnings, err := e.evalQuery(string(policy), "data.fiac.warn", input)
		if err == nil {
			result.Warnings = append(result.Warnings, warnings...)
		}
	}

	result.Passed = len(result.Denials) == 0
	return result, nil
}

func (e *Evaluator) evalQuery(policy, query string, input map[string]any) ([]string, error) {
	ctx := context.Background()
	r := rego.New(
		rego.Query(query),
		rego.Module("policy.rego", policy),
		rego.Input(input),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		return nil, err
	}

	var messages []string
	for _, result := range rs {
		for _, expr := range result.Expressions {
			if set, ok := expr.Value.([]interface{}); ok {
				for _, v := range set {
					if msg, ok := v.(string); ok {
						messages = append(messages, msg)
					}
				}
			}
		}
	}
	return messages, nil
}

func (e *Evaluator) ValidatePolicies() error {
	files, err := filepath.Glob(filepath.Join(e.policiesDir, "*.rego"))
	if err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		_, err = rego.New(rego.Module(file, string(content))).PrepareForEval(context.Background())
		if err != nil {
			return fmt.Errorf("invalid policy %s: %w", file, err)
		}
	}
	return nil
}
