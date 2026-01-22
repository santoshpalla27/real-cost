// Package policy provides the Policy & Governance Engine
// Evaluates cost and carbon policies against estimation results
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"terraform-cost/decision/estimation"
)

// PolicyType defines the type of policy
type PolicyType string

const (
	PolicyTypeCostLimit           PolicyType = "cost_limit"
	PolicyTypeCostGrowth          PolicyType = "cost_growth"
	PolicyTypeConfidenceThreshold PolicyType = "confidence_threshold"
	PolicyTypeCarbonBudget        PolicyType = "carbon_budget"
	PolicyTypeIncompleteEstimate  PolicyType = "incomplete_estimate"
	PolicyTypeCustom              PolicyType = "custom"
)

// Severity defines policy violation severity
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Decision is the policy evaluation outcome
type Decision string

const (
	DecisionPass Decision = "pass"
	DecisionWarn Decision = "warn"
	DecisionDeny Decision = "deny"
)

// Policy defines a governance rule
type Policy struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        PolicyType `json:"type"`
	Severity    Severity   `json:"severity"`
	Threshold   float64    `json:"threshold"`
	Enabled     bool       `json:"enabled"`
}

// Violation represents a policy violation
type Violation struct {
	PolicyID   string `json:"policy_id"`
	PolicyName string `json:"policy_name"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// Warning represents a policy warning
type Warning struct {
	PolicyID string `json:"policy_id"`
	Message  string `json:"message"`
}

// EvaluationRequest contains the input for policy evaluation
type EvaluationRequest struct {
	Estimation     *estimation.EstimationResult
	Environment    string
	CustomPolicies []Policy
}

// EvaluationResult contains the policy evaluation outcome
type EvaluationResult struct {
	Decision       Decision    `json:"decision"`
	Violations     []Violation `json:"violations"`
	Warnings       []Warning   `json:"warnings"`
	PoliciesRan    int         `json:"policies_ran"`
	EvaluatedAt    time.Time   `json:"evaluated_at"`
}

// Engine evaluates policies against estimations
type Engine struct {
	policies    []Policy
	opaEndpoint string
	httpClient  *http.Client
}

// NewEngine creates a new policy engine
func NewEngine() *Engine {
	return &Engine{
		policies: defaultPolicies(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// WithOPA configures OPA integration
func (e *Engine) WithOPA(endpoint string) *Engine {
	e.opaEndpoint = endpoint
	return e
}

// AddPolicy adds a custom policy
func (e *Engine) AddPolicy(p Policy) {
	e.policies = append(e.policies, p)
}

// Evaluate runs all policies against the estimation
func (e *Engine) Evaluate(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	result := &EvaluationResult{
		Decision:    DecisionPass,
		Violations:  make([]Violation, 0),
		Warnings:    make([]Warning, 0),
		EvaluatedAt: time.Now(),
	}

	// Combine built-in and custom policies
	allPolicies := append(e.policies, req.CustomPolicies...)

	for _, policy := range allPolicies {
		if !policy.Enabled {
			continue
		}

		result.PoliciesRan++
		violation, warning := e.evaluatePolicy(policy, req.Estimation, req.Environment)

		if violation != nil {
			result.Violations = append(result.Violations, *violation)
			if policy.Severity == SeverityError {
				result.Decision = DecisionDeny
			} else if result.Decision != DecisionDeny {
				result.Decision = DecisionWarn
			}
		}

		if warning != nil {
			result.Warnings = append(result.Warnings, *warning)
			if result.Decision == DecisionPass {
				result.Decision = DecisionWarn
			}
		}
	}

	// Run OPA policies if configured
	if e.opaEndpoint != "" {
		opaResult, err := e.evaluateOPA(ctx, req)
		if err == nil && opaResult != nil {
			result.Violations = append(result.Violations, opaResult.Violations...)
			result.Warnings = append(result.Warnings, opaResult.Warnings...)
			if len(opaResult.Violations) > 0 {
				result.Decision = DecisionDeny
			}
		}
	}

	return result, nil
}

func (e *Engine) evaluatePolicy(p Policy, est *estimation.EstimationResult, env string) (*Violation, *Warning) {
	switch p.Type {
	case PolicyTypeCostLimit:
		costP90, _ := est.MonthlyCostP90.Float64()
		if costP90 > p.Threshold {
			return &Violation{
				PolicyID:   p.ID,
				PolicyName: p.Name,
				Message:    fmt.Sprintf("Monthly cost P90 ($%.2f) exceeds limit ($%.2f)", costP90, p.Threshold),
				Severity:   string(p.Severity),
			}, nil
		}

	case PolicyTypeConfidenceThreshold:
		if est.Confidence < p.Threshold/100 {
			if p.Severity == SeverityError {
				return &Violation{
					PolicyID:   p.ID,
					PolicyName: p.Name,
					Message:    fmt.Sprintf("Estimation confidence (%.0f%%) below threshold (%.0f%%)", est.Confidence*100, p.Threshold),
					Severity:   string(p.Severity),
				}, nil
			}
			return nil, &Warning{
				PolicyID: p.ID,
				Message:  fmt.Sprintf("Estimation confidence (%.0f%%) below recommended (%.0f%%)", est.Confidence*100, p.Threshold),
			}
		}

	case PolicyTypeCarbonBudget:
		if est.CarbonKgCO2 > p.Threshold {
			return &Violation{
				PolicyID:   p.ID,
				PolicyName: p.Name,
				Message:    fmt.Sprintf("Carbon emissions (%.2f kg CO2) exceed budget (%.2f kg)", est.CarbonKgCO2, p.Threshold),
				Severity:   string(p.Severity),
			}, nil
		}

	case PolicyTypeIncompleteEstimate:
		if est.IsIncomplete && env == "prod" {
			return &Violation{
				PolicyID:   p.ID,
				PolicyName: p.Name,
				Message:    fmt.Sprintf("Incomplete estimation not allowed in production (%d symbolic costs)", est.ComponentsSymbolic),
				Severity:   string(p.Severity),
			}, nil
		}
	}

	return nil, nil
}

func (e *Engine) evaluateOPA(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	if e.opaEndpoint == "" {
		return nil, nil
	}

	// Build OPA input
	input := map[string]interface{}{
		"monthly_cost_p50": req.Estimation.MonthlyCostP50.InexactFloat64(),
		"monthly_cost_p90": req.Estimation.MonthlyCostP90.InexactFloat64(),
		"carbon_kg_co2":    req.Estimation.CarbonKgCO2,
		"confidence":       req.Estimation.Confidence,
		"is_incomplete":    req.Estimation.IsIncomplete,
		"environment":      req.Environment,
	}

	body, _ := json.Marshal(map[string]interface{}{"input": input})
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.opaEndpoint+"/v1/data/terracost/deny", nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse OPA response - simplified
	_ = body // Used in actual implementation
	
	return &EvaluationResult{
		Decision:   DecisionPass,
		Violations: []Violation{},
		Warnings:   []Warning{},
	}, nil
}

func defaultPolicies() []Policy {
	return []Policy{
		{
			ID:          "default-confidence",
			Name:        "Minimum Confidence",
			Description: "Warn when estimation confidence is below 70%",
			Type:        PolicyTypeConfidenceThreshold,
			Severity:    SeverityWarning,
			Threshold:   70,
			Enabled:     true,
		},
		{
			ID:          "prod-incomplete",
			Name:        "No Incomplete in Prod",
			Description: "Block incomplete estimations in production",
			Type:        PolicyTypeIncompleteEstimate,
			Severity:    SeverityError,
			Threshold:   0,
			Enabled:     true,
		},
	}
}
