package cost.governance

default allow = false

# External data passed in 'input.budget'
budget_limit = input.budget.total_budget


allow {
	count(violations) == 0
}

violation[msg] {
	input.estimate.total_monthly_cost.p90 > budget_limit
	msg := sprintf("Total P90 monthly cost $%.2f exceeds budget of $%.2f", [input.estimate.total_monthly_cost.p90, budget_limit])
}

violations[msg] {
	# Example: Carbon Intensity Check
	# input.estimate.carbon.region_intensity == "high"
	# msg := "Deployment in high carbon intensity region detected"
	false
}

violations[msg] {
	# Check for critical errors or incompleteness
	input.estimate.is_incomplete == true
	msg := sprintf("Estimation is incomplete due to missing data: %v", [input.estimate.errors])
}

violations[msg] {
	# Confidence must be high enough for approval
	input.estimate.confidence_score < 0.8
	msg := sprintf("Estimation confidence too low (%.2f < 0.8) for approval", [input.estimate.confidence_score])
}

warnings[msg] {
	input.estimate.total_monthly_cost.p90 > 800
	msg := sprintf("Approaching budget limit: $%.2f > $800", [input.estimate.total_monthly_cost.p90])
}
