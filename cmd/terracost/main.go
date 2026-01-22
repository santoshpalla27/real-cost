// TerraCost CLI - IaC Cost Intelligence Platform
//
// Usage:
//   terracost estimate --plan plan.json [options]
//   terracost pricing update --provider aws --region us-east-1
//   terracost policy evaluate --plan plan.json
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"

	"terraform-cost/api"
	"terraform-cost/db/clickhouse"
	"terraform-cost/decision/billing"
	"terraform-cost/decision/billing/mappers/aws"
	"terraform-cost/decision/estimation"
	"terraform-cost/decision/iac"
	"terraform-cost/decision/policy"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := &cli.App{
		Name:    "terracost",
		Usage:   "IaC Cost Intelligence Platform - Shift-Left Financial Control for Terraform",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Log level (debug, info, warn, error)",
				EnvVars: []string{"TERRACOST_LOG_LEVEL"},
			},
			&cli.StringFlag{
				Name:    "clickhouse-host",
				Value:   "localhost",
				Usage:   "ClickHouse host",
				EnvVars: []string{"CLICKHOUSE_HOST"},
			},
			&cli.IntFlag{
				Name:    "clickhouse-port",
				Value:   9000,
				Usage:   "ClickHouse native port",
				EnvVars: []string{"CLICKHOUSE_PORT"},
			},
			&cli.StringFlag{
				Name:    "clickhouse-database",
				Value:   "terracost",
				Usage:   "ClickHouse database",
				EnvVars: []string{"CLICKHOUSE_DATABASE"},
			},
			&cli.StringFlag{
				Name:    "clickhouse-user",
				Value:   "default",
				Usage:   "ClickHouse user",
				EnvVars: []string{"CLICKHOUSE_USER"},
			},
			&cli.StringFlag{
				Name:    "clickhouse-password",
				Value:   "",
				Usage:   "ClickHouse password",
				EnvVars: []string{"CLICKHOUSE_PASSWORD"},
			},
		},
		
		Commands: []*cli.Command{
			estimateCommand(),
			serveCommand(),
			pricingCommand(),
			policyCommand(),
		},
	}
	
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// =============================================================================
// ESTIMATE COMMAND
// =============================================================================

func estimateCommand() *cli.Command {
	return &cli.Command{
		Name:  "estimate",
		Usage: "Estimate cost and carbon for a Terraform plan",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "plan",
				Aliases:  []string{"p"},
				Usage:    "Path to terraform plan JSON (from terraform show -json)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Value:   "dev",
				Usage:   "Environment (dev, staging, prod)",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "table",
				Usage:   "Output format (table, json, markdown)",
			},
			&cli.Float64Flag{
				Name:  "cost-limit",
				Usage: "Monthly cost limit for policy check",
			},
			&cli.Float64Flag{
				Name:  "carbon-budget",
				Usage: "Carbon budget (kg CO2) for policy check",
			},
			&cli.BoolFlag{
				Name:  "include-carbon",
				Value: false,
				Usage: "Include carbon emissions in output",
			},
			&cli.BoolFlag{
				Name:  "include-formulas",
				Value: false,
				Usage: "Include cost formulas in output",
			},
			&cli.BoolFlag{
				Name:  "skip-policy",
				Value: false,
				Usage: "Skip policy evaluation",
			},
			&cli.StringFlag{
				Name:  "opa-endpoint",
				Usage: "OPA endpoint for policy evaluation",
			},
		},
		Action: runEstimate,
	}
}

func runEstimate(c *cli.Context) error {
	ctx := context.Background()
	
	// Parse Terraform plan
	parser := iac.NewParser()
	plan, err := parser.ParseFile(c.String("plan"))
	if err != nil {
		return fmt.Errorf("failed to parse terraform plan: %w", err)
	}
	
	// Build infrastructure graph
	graphBuilder := iac.NewGraphBuilder()
	graph, err := graphBuilder.Build(plan)
	if err != nil {
		return fmt.Errorf("failed to build infrastructure graph: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "📊 Parsed %d resources (%d creates, %d updates, %d deletes)\n",
		graph.ResourceCount,
		graph.ChangeStats.Creates,
		graph.ChangeStats.Updates,
		graph.ChangeStats.Deletes,
	)
	
	// Initialize billing engine
	billingEngine := billing.NewEngine()
	aws.RegisterAllMappers(billingEngine)
	
	// Decompose resources into billing components
	decomposition, err := billingEngine.Decompose(graph)
	if err != nil {
		return fmt.Errorf("failed to decompose resources: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "💰 Generated %d billing components from %d resources\n",
		decomposition.ComponentsCreated,
		decomposition.ResourcesMapped,
	)
	
	if len(decomposition.UncoveredTypes) > 0 {
		fmt.Fprintf(os.Stderr, "⚠️  Unsupported resource types: %s\n",
			strings.Join(decomposition.UncoveredTypes, ", "))
	}
	
	// Connect to ClickHouse
	store, err := clickhouse.NewStore(&clickhouse.Config{
		Host:     c.String("clickhouse-host"),
		Port:     c.Int("clickhouse-port"),
		Database: c.String("clickhouse-database"),
		Username: c.String("clickhouse-user"),
		Password: c.String("clickhouse-password"),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}
	defer store.Close()
	
	// Run estimation
	estimationEngine := estimation.NewEngine(store)
	
	result, err := estimationEngine.Estimate(ctx, estimation.EstimationRequest{
		Components:      decomposition.Components,
		Environment:     c.String("env"),
		IncludeCarbon:   c.Bool("include-carbon"),
		IncludeFormulas: c.Bool("include-formulas"),
	})
	if err != nil {
		return fmt.Errorf("estimation failed: %w", err)
	}
	
	// Run policy evaluation
	var policyResult *policy.EvaluationResult
	if !c.Bool("skip-policy") {
		policyEngine := policy.NewEngine()
		
		// Add custom policies from flags
		if limit := c.Float64("cost-limit"); limit > 0 {
			policyEngine.AddPolicy(policy.Policy{
				ID:        "cli-cost-limit",
				Name:      "Cost Limit",
				Type:      policy.PolicyTypeCostLimit,
				Severity:  policy.SeverityError,
				Threshold: limit,
				Enabled:   true,
			})
		}
		
		if budget := c.Float64("carbon-budget"); budget > 0 {
			policyEngine.AddPolicy(policy.Policy{
				ID:        "cli-carbon-budget",
				Name:      "Carbon Budget",
				Type:      policy.PolicyTypeCarbonBudget,
				Severity:  policy.SeverityError,
				Threshold: budget,
				Enabled:   true,
			})
		}
		
		// Configure OPA if endpoint provided
		if opaEndpoint := c.String("opa-endpoint"); opaEndpoint != "" {
			policyEngine.WithOPA(opaEndpoint)
		}
		
		policyResult, err = policyEngine.Evaluate(ctx, policy.EvaluationRequest{
			Estimation:  result,
			Environment: c.String("env"),
		})
		if err != nil {
			return fmt.Errorf("policy evaluation failed: %w", err)
		}
	}
	
	// Output results
	switch c.String("format") {
	case "json":
		return outputJSON(result, policyResult)
	case "markdown":
		return outputMarkdown(result, policyResult)
	default:
		return outputTable(result, policyResult)
	}
}

// =============================================================================
// OUTPUT FORMATTERS
// =============================================================================

type JSONOutput struct {
	MonthlyCostP50     string               `json:"monthly_cost_p50"`
	MonthlyCostP90     string               `json:"monthly_cost_p90"`
	CarbonKgCO2        float64              `json:"carbon_kg_co2"`
	Confidence         float64              `json:"confidence"`
	IsIncomplete       bool                 `json:"is_incomplete"`
	ResourceCount      int                  `json:"resource_count"`
	ComponentsEstimated int                 `json:"components_estimated"`
	ComponentsSymbolic int                  `json:"components_symbolic"`
	PolicyResult       string               `json:"policy_result"`
	Violations         []policy.Violation   `json:"violations,omitempty"`
	Warnings           []policy.Warning     `json:"warnings,omitempty"`
	CostDrivers        []estimation.CostDriver `json:"cost_drivers"`
}

func outputJSON(result *estimation.EstimationResult, policyResult *policy.EvaluationResult) error {
	output := JSONOutput{
		MonthlyCostP50:     result.MonthlyCostP50.StringFixed(2),
		MonthlyCostP90:     result.MonthlyCostP90.StringFixed(2),
		CarbonKgCO2:        result.CarbonKgCO2,
		Confidence:         result.Confidence,
		IsIncomplete:       result.IsIncomplete,
		ResourceCount:      result.ComponentsProcessed,
		ComponentsEstimated: result.ComponentsEstimated,
		ComponentsSymbolic: result.ComponentsSymbolic,
		CostDrivers:        result.CostDrivers,
	}
	
	if policyResult != nil {
		output.PolicyResult = string(policyResult.Decision)
		output.Violations = policyResult.Violations
		output.Warnings = policyResult.Warnings
	}
	
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputTable(result *estimation.EstimationResult, policyResult *policy.EvaluationResult) error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    💰 COST ESTIMATION                         ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Monthly Cost (P50):    $%-37s ║\n", result.MonthlyCostP50.StringFixed(2))
	fmt.Printf("║  Monthly Cost (P90):    $%-37s ║\n", result.MonthlyCostP90.StringFixed(2))
	fmt.Printf("║  Hourly Cost:           $%-37s ║\n", result.HourlyCostP50.StringFixed(4))
	fmt.Printf("║  Confidence:            %-38s ║\n", fmt.Sprintf("%.0f%%", result.Confidence*100))
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	
	// Top cost drivers
	fmt.Println("║  TOP COST DRIVERS                                             ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	
	maxDrivers := 5
	if len(result.CostDrivers) < maxDrivers {
		maxDrivers = len(result.CostDrivers)
	}
	
	for i := 0; i < maxDrivers; i++ {
		driver := result.CostDrivers[i]
		name := truncate(driver.Description, 35)
		cost := driver.MonthlyCostP50.StringFixed(2)
		fmt.Printf("║  %-35s  $%-20s ║\n", name, cost)
	}
	
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	
	// Policy result
	if policyResult != nil {
		var policyIcon string
		switch policyResult.Decision {
		case policy.DecisionPass:
			policyIcon = "✅ PASS"
		case policy.DecisionWarn:
			policyIcon = "⚠️  WARN"
		case policy.DecisionDeny:
			policyIcon = "❌ DENY"
		}
		fmt.Printf("║  Policy Result:         %-38s ║\n", policyIcon)
		
		for _, v := range policyResult.Violations {
			fmt.Printf("║  ❌ %-57s ║\n", truncate(v.Message, 57))
		}
		for _, w := range policyResult.Warnings {
			fmt.Printf("║  ⚠️  %-56s ║\n", truncate(w.Message, 56))
		}
	}
	
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	
	// Return appropriate exit code
	if policyResult != nil && policyResult.Decision == policy.DecisionDeny {
		os.Exit(2)
	}
	
	return nil
}

func outputMarkdown(result *estimation.EstimationResult, policyResult *policy.EvaluationResult) error {
	fmt.Println("## 💰 TerraCost Estimation Report")
	fmt.Println()
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	fmt.Printf("| **Monthly Cost (P50)** | $%s |\n", result.MonthlyCostP50.StringFixed(2))
	fmt.Printf("| **Monthly Cost (P90)** | $%s |\n", result.MonthlyCostP90.StringFixed(2))
	fmt.Printf("| **Confidence** | %.0f%% |\n", result.Confidence*100)
	
	if result.CarbonKgCO2 > 0 {
		fmt.Printf("| **Carbon Emissions** | %.2f kg CO2 |\n", result.CarbonKgCO2)
	}
	
	if policyResult != nil {
		fmt.Printf("| **Policy Result** | %s |\n", policyResult.Decision)
	}
	
	fmt.Println()
	fmt.Println("### 📊 Cost Breakdown")
	fmt.Println()
	fmt.Println("| Resource | Service | Monthly Cost |")
	fmt.Println("|----------|---------|--------------|")
	
	for _, driver := range result.CostDrivers {
		if driver.MonthlyCostP50.GreaterThan(decimal.Zero) || driver.IsSymbolic {
			cost := "$" + driver.MonthlyCostP50.StringFixed(2)
			if driver.IsSymbolic {
				cost = "⚠️ Unknown"
			}
			fmt.Printf("| %s | %s | %s |\n", driver.ResourceAddr, driver.Service, cost)
		}
	}
	
	if policyResult != nil && len(policyResult.Violations) > 0 {
		fmt.Println()
		fmt.Println("### ❌ Policy Violations")
		fmt.Println()
		for _, v := range policyResult.Violations {
			fmt.Printf("- **%s**: %s\n", v.PolicyName, v.Message)
		}
	}
	
	if policyResult != nil && len(policyResult.Warnings) > 0 {
		fmt.Println()
		fmt.Println("### ⚠️ Warnings")
		fmt.Println()
		for _, w := range policyResult.Warnings {
			fmt.Printf("- %s\n", w.Message)
		}
	}
	
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// =============================================================================
// PRICING COMMAND
// =============================================================================

func pricingCommand() *cli.Command {
	return &cli.Command{
		Name:  "pricing",
		Usage: "Manage pricing data",
		Subcommands: []*cli.Command{
			{
				Name:  "update",
				Usage: "Update pricing data from cloud providers",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "provider",
						Usage:    "Cloud provider (aws, azure, gcp)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "region",
						Value: "us-east-1",
						Usage: "Region (or 'all' for all regions)",
					},
					&cli.StringFlag{
						Name:  "memory-profile",
						Value: "normal",
						Usage: "Memory profile (low, normal, high)",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Value: false,
						Usage: "Dry run (no database writes)",
					},
				},
				Action: func(c *cli.Context) error {
					fmt.Println("Pricing update not yet implemented in this version")
					fmt.Println("Use the existing pricing ingestion commands")
					return nil
				},
			},
			{
				Name:  "validate",
				Usage: "Validate pricing coverage",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "provider",
						Usage:    "Cloud provider",
						Required: true,
					},
					&cli.IntFlag{
						Name:  "min-coverage",
						Value: 80,
						Usage: "Minimum coverage percentage",
					},
				},
				Action: func(c *cli.Context) error {
					fmt.Println("Pricing validation not yet implemented")
					return nil
				},
			},
		},
	}
}

// =============================================================================
// POLICY COMMAND
// =============================================================================

func policyCommand() *cli.Command {
	return &cli.Command{
		Name:  "policy",
		Usage: "Manage policies",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List available policies",
				Action: func(c *cli.Context) error {
					fmt.Println("Built-in Policies:")
					fmt.Println("  - cost_limit: Maximum monthly cost threshold")
					fmt.Println("  - cost_growth: Maximum cost increase percentage")
					fmt.Println("  - confidence_threshold: Minimum estimation confidence")
					fmt.Println("  - carbon_budget: Maximum carbon emissions")
					fmt.Println("  - incomplete_estimate: Block on incomplete estimations")
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "Test policies against a plan",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "plan",
						Usage:    "Path to terraform plan JSON",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "policy-file",
						Usage: "Path to custom policy file",
					},
				},
				Action: func(c *cli.Context) error {
					fmt.Println("Policy testing not yet implemented")
					return nil
				},
			},
		},
	}
}

// =============================================================================
// SERVE COMMAND (API SERVER)
// =============================================================================

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the TerraCost API server",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Value:   8080,
				Usage:   "API server port",
				EnvVars: []string{"TERRACOST_PORT"},
			},
			&cli.StringFlag{
				Name:    "cors-origins",
				Value:   "*",
				Usage:   "Comma-separated list of allowed CORS origins",
				EnvVars: []string{"TERRACOST_CORS_ORIGINS"},
			},
			&cli.StringFlag{
				Name:    "opa-endpoint",
				Usage:   "OPA endpoint for policy evaluation",
				EnvVars: []string{"OPA_ENDPOINT"},
			},
		},
		Action: runServe,
	}
}

func runServe(c *cli.Context) error {
	// Connect to ClickHouse
	store, err := clickhouse.NewStore(&clickhouse.Config{
		Host:     c.String("clickhouse-host"),
		Port:     c.Int("clickhouse-port"),
		Database: c.String("clickhouse-database"),
		Username: c.String("clickhouse-user"),
		Password: c.String("clickhouse-password"),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}
	defer store.Close()

	// Parse CORS origins
	corsOrigins := strings.Split(c.String("cors-origins"), ",")
	for i := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
	}

	// Create and start API server
	server := api.NewServer(store, &api.Config{
		Port:        c.Int("port"),
		CORSOrigins: corsOrigins,
		OPAEndpoint: c.String("opa-endpoint"),
	})

	return server.StartWithGracefulShutdown()
}

