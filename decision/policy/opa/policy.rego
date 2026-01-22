# TerraCost Policy Engine
#
# This Rego package defines policies for infrastructure cost governance.
# Policies evaluate estimation results and can deny, warn, or allow deployments.

package terracost.policy

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Default allow
default allow := true

# =============================================================================
# DENY RULES - These block deployment
# =============================================================================

# Deny if monthly cost exceeds absolute limit
deny contains msg if {
    input.monthly_cost_p90 > data.limits.max_monthly_cost
    msg := sprintf("Monthly cost P90 ($%.2f) exceeds limit ($%.2f)", [input.monthly_cost_p90, data.limits.max_monthly_cost])
}

# Deny if carbon emissions exceed budget
deny contains msg if {
    data.limits.carbon_budget_kg != null
    input.carbon_kg_co2 > data.limits.carbon_budget_kg
    msg := sprintf("Carbon emissions (%.2f kg CO2) exceed budget (%.2f kg)", [input.carbon_kg_co2, data.limits.carbon_budget_kg])
}

# Deny if confidence is too low for production
deny contains msg if {
    input.environment == "production"
    input.confidence < 0.7
    msg := sprintf("Estimation confidence (%.0f%%) too low for production (minimum 70%%)", [input.confidence * 100])
}

# Deny if too many resources cannot be priced
deny contains msg if {
    input.symbolic_count > data.limits.max_symbolic_resources
    msg := sprintf("Too many unpriced resources (%d exceeds limit of %d)", [input.symbolic_count, data.limits.max_symbolic_resources])
}

# =============================================================================
# WARN RULES - These generate warnings but allow deployment
# =============================================================================

# Warn if cost is high even if under limit
warn contains msg if {
    input.monthly_cost_p50 > data.thresholds.cost_review_threshold
    msg := sprintf("Monthly cost ($%.2f) is above review threshold ($%.2f) - consider cost optimization", [input.monthly_cost_p50, data.thresholds.cost_review_threshold])
}

# Warn if using high-cost regions
warn contains msg if {
    some region in input.regions
    region in data.high_cost_regions
    msg := sprintf("Using high-cost region: %s - consider alternatives", [region])
}

# Warn if single service dominates cost
warn contains msg if {
    some service, cost in input.costs_by_service
    cost / input.monthly_cost_p50 > 0.8
    msg := sprintf("Service %s accounts for >80%% of costs - review for optimization", [service])
}

# Warn on low confidence
warn contains msg if {
    input.confidence < 0.5
    msg := sprintf("Low estimation confidence (%.0f%%) - costs may vary significantly", [input.confidence * 100])
}

# Warn if any resources are incomplete
warn contains msg if {
    input.is_incomplete == true
    msg := sprintf("Estimation is incomplete (%d resources could not be priced)", [input.symbolic_count])
}

# =============================================================================
# DATA - Default configuration (can be overridden)
# =============================================================================

# Default limits
limits := {
    "max_monthly_cost": 10000,
    "carbon_budget_kg": null,
    "max_symbolic_resources": 5
}

# Default thresholds
thresholds := {
    "cost_review_threshold": 1000
}

# High cost regions (example)
high_cost_regions := [
    "ap-northeast-1",  # Tokyo
    "eu-west-1",       # Ireland
    "ap-southeast-1"   # Singapore
]
