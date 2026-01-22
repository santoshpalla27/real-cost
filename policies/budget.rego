package fiac

# Budget policy - deny if cost exceeds threshold

deny[msg] {
    input.total_cost_p90 > 10000
    msg := sprintf("Monthly cost P90 ($%.2f) exceeds $10,000 budget", [input.total_cost_p90])
}

deny[msg] {
    input.cost_growth_percent > 20
    msg := sprintf("Cost growth %.1f%% exceeds 20%% limit", [input.cost_growth_percent])
}

warn[msg] {
    input.total_cost_p90 > 5000
    input.total_cost_p90 <= 10000
    msg := sprintf("Monthly cost P90 ($%.2f) approaching budget limit", [input.total_cost_p90])
}
