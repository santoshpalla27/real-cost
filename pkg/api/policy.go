package api

// PolicyResult contains the governance decision.
type PolicyResult struct {
	Allowed    bool     `json:"allowed"`
	Violations []string `json:"violations"`
	Warnings   []string `json:"warnings"`
}

// PolicyRequest wraps the input for OPA.
type PolicyRequest struct {
	Input PolicyInput `json:"input"`
}

type PolicyInput struct {
	Estimate *EstimationResult `json:"estimate"`
	Budget   *BudgetInfo       `json:"budget"`
}

type BudgetInfo struct {
	TotalBudget float64 `json:"total_budget"`
}
