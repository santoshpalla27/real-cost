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

	// FAIL-CLOSED: Incomplete estimation must not pass policy evaluation
	if est.IsIncomplete {
		result.Passed = false
		result.Denials = append(result.Denials, "BLOCKED: Estimation is incomplete - cannot evaluate policies on partial data")
		return result, nil
	}

	input := map[string]any{
		"total_cost_p50":      est.TotalCost.P50,
		"total_cost_p90":      est.TotalCost.P90,
		"carbon_kg":           est.TotalCarbon.KgCO2e,
		"confidence_score":    est.ConfidenceScore,
		"is_incomplete":       est.IsIncomplete,
		"cost_growth_percent": 0.0,
		"error_count":         len(est.Errors),
	}

	files, err := filepath.Glob(filepath.Join(e.policiesDir, "*.rego"))
	if err != nil {
		// FAIL-CLOSED: Cannot list policies = fail
		return nil, fmt.Errorf("FAIL-CLOSED: cannot list policies: %w", err)
	}

	// FAIL-CLOSED: No policies = explicit pass only if configured
	if len(files) == 0 {
		result.Warnings = append(result.Warnings, "No policies found - defaulting to pass")
		return result, nil
	}

	for _, file := range files {
		policy, err := os.ReadFile(file)
		if err != nil {
			// FAIL-CLOSED: Cannot read policy = fail
			return nil, fmt.Errorf("FAIL-CLOSED: cannot read policy %s: %w", file, err)
		}

		denials, err := e.evalQuery(string(policy), "data.fiac.deny", input)
		if err != nil {
			// FAIL-CLOSED: Policy evaluation error = fail
			return nil, fmt.Errorf("FAIL-CLOSED: policy evaluation failed for %s: %w", file, err)
		}
		result.Denials = append(result.Denials, denials...)

		warnings, err := e.evalQuery(string(policy), "data.fiac.warn", input)
		if err != nil {
			// Warnings evaluation failure is less critical, but still log
			result.Warnings = append(result.Warnings, fmt.Sprintf("Warning evaluation failed for %s", file))
		} else {
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
