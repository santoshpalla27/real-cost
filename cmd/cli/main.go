// Package main provides the FIAC CLI tool for developers.
// This tool orchestrates service calls and provides CI-safe output.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/santoshpalla27/fiac-platform/internal/estimation"
	"github.com/santoshpalla27/fiac-platform/internal/graph"
	"github.com/santoshpalla27/fiac-platform/internal/policy"
	"github.com/santoshpalla27/fiac-platform/internal/pricing"
	"github.com/santoshpalla27/fiac-platform/internal/semantics"
	"github.com/santoshpalla27/fiac-platform/internal/usage"
)

// Exit codes for CI/CD integration
const (
	ExitSuccess       = 0
	ExitPolicyDeny    = 1
	ExitPolicyWarn    = 2
	ExitParseError    = 10
	ExitEstimateError = 11
	ExitIncomplete    = 20
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Define subcommands
	estimateCmd := flag.NewFlagSet("estimate", flag.ExitOnError)
	planFile := estimateCmd.String("plan", "", "Path to Terraform plan JSON file")
	outputFormat := estimateCmd.String("output", "text", "Output format: text, json")
	environment := estimateCmd.String("env", "dev", "Environment: dev, staging, prod")
	policiesDir := estimateCmd.String("policies", "policies", "Path to OPA policies directory")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(ExitParseError)
	}

	switch os.Args[1] {
	case "estimate":
		estimateCmd.Parse(os.Args[2:])
		exitCode := runEstimate(*planFile, *outputFormat, *environment, *policiesDir)
		os.Exit(exitCode)
	case "version":
		fmt.Println("fiac v0.1.0")
		os.Exit(ExitSuccess)
	case "help", "-h", "--help":
		printUsage()
		os.Exit(ExitSuccess)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(ExitParseError)
	}
}

func printUsage() {
	fmt.Println(`FIAC - IaC Cost Intelligence Platform

Usage:
  fiac <command> [options]

Commands:
  estimate    Estimate costs for a Terraform plan
  version     Print version information
  help        Show this help message

Estimate Options:
  --plan      Path to Terraform plan JSON file (required)
  --output    Output format: text, json (default: text)
  --env       Environment: dev, staging, prod (default: dev)
  --policies  Path to OPA policies directory (default: policies)

Examples:
  fiac estimate --plan tfplan.json --output json
  fiac estimate --plan tfplan.json --env prod`)
}

func runEstimate(planFile, outputFormat, environment, policiesDir string) int {
	if planFile == "" {
		log.Error().Msg("--plan flag is required")
		return ExitParseError
	}

	// Read plan file
	planData, err := os.ReadFile(planFile)
	if err != nil {
		log.Error().Err(err).Str("file", planFile).Msg("Failed to read plan file")
		return ExitParseError
	}

	// Step 1: Parse Terraform plan
	log.Info().Msg("Parsing Terraform plan...")
	infraGraph, err := graph.ParseTerraformPlan(planData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse Terraform plan")
		return ExitParseError
	}
	log.Info().Int("resources", len(infraGraph.Nodes)).Msg("Infrastructure graph built")

	// Step 2: Extract billing semantics
	log.Info().Msg("Extracting billing semantics...")
	semanticEngine := semantics.NewEngine()
	billingResult, err := semanticEngine.Process(infraGraph)
	if err != nil {
		log.Error().Err(err).Msg("Failed to extract billing semantics")
		return ExitEstimateError
	}
	log.Info().
		Int("components", len(billingResult.Components)).
		Int("errors", len(billingResult.MappingErrors)).
		Msg("Billing components extracted")

	// Step 3: Predict usage
	log.Info().Msg("Predicting usage...")
	predictor := usage.NewPredictor()
	usageResult := predictor.Predict(billingResult.Components, environment)

	// FAIL-CLOSED: Unknown environment is a fatal error
	if usageResult.UnknownEnvironment {
		log.Error().Str("env", environment).Msg("FAIL-CLOSED: Unknown environment")
		log.Error().Msg(usageResult.EnvironmentError)
		return ExitEstimateError
	}
	log.Info().Float64("avg_confidence", usageResult.AverageConfidence).Msg("Usage predicted")

	// Step 4: Resolve pricing
	log.Info().Msg("Resolving pricing...")
	resolver := pricing.NewResolver()
	priceResult, err := resolver.Resolve(billingResult.Components, infraGraph.ProviderContext.Region, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to resolve pricing")
		return ExitEstimateError
	}
	log.Info().Int("prices", len(priceResult.Prices)).Msg("Prices resolved")

	// Step 5: Calculate estimation
	log.Info().Msg("Calculating cost estimation...")
	calculator := estimation.NewCalculator()
	estimateResult, err := calculator.CalculateFromComponents(
		billingResult,
		usageResult,
		priceResult,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate estimation")
		return ExitEstimateError
	}

	// Step 6: Evaluate policies
	log.Info().Msg("Evaluating policies...")
	evaluator := policy.NewEvaluator(policiesDir)
	policyResult, err := evaluator.Evaluate(estimateResult)
	
	// FAIL-CLOSED: Policy evaluation errors are fatal
	if err != nil {
		log.Error().Err(err).Msg("FAIL-CLOSED: Policy evaluation failed")
		return ExitPolicyDeny
	}

	// Output results
	if outputFormat == "json" {
		outputJSON(estimateResult, policyResult)
	} else {
		outputText(estimateResult, policyResult)
	}

	// Determine exit code - fail-closed order
	if estimateResult.IsIncomplete {
		return ExitIncomplete
	}
	if policyResult != nil && len(policyResult.Denials) > 0 {
		return ExitPolicyDeny
	}
	if policyResult != nil && len(policyResult.Warnings) > 0 {
		return ExitPolicyWarn
	}
	return ExitSuccess
}

func outputJSON(result *estimation.Result, policyResult *policy.Result) {
	output := map[string]any{
		"estimation": result,
	}
	if policyResult != nil {
		output["policy"] = policyResult
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func outputText(result *estimation.Result, policyResult *policy.Result) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              FIAC Cost Estimation Report                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")

	fmt.Printf("\n📊 Cost Estimate\n")
	fmt.Printf("   P50 (Median):    $%.2f/month\n", result.TotalCost.P50)
	fmt.Printf("   P90 (Pessimistic): $%.2f/month\n", result.TotalCost.P90)

	fmt.Printf("\n🌱 Carbon Footprint\n")
	fmt.Printf("   Estimated:       %.2f kgCO2e/month\n", result.TotalCarbon.KgCO2e)

	fmt.Printf("\n🎯 Confidence Score: %.0f%%\n", result.ConfidenceScore*100)

	if result.IsIncomplete {
		fmt.Println("\n⚠️  INCOMPLETE ESTIMATE")
		fmt.Println("   Some resources could not be mapped. Total costs set to 0.")
		for _, err := range result.Errors {
			fmt.Printf("   • %s: %s\n", err.ResourceID, err.Message)
		}
	}

	if len(result.Drivers) > 0 {
		fmt.Println("\n💰 Cost Drivers")
		for _, d := range result.Drivers {
			fmt.Printf("   • %s: $%.2f (%.0f%%)\n", d.Name, d.MonthlyCost, d.Percentage)
		}
	}

	if policyResult != nil {
		if len(policyResult.Denials) > 0 {
			fmt.Println("\n🚫 POLICY VIOLATIONS (Blocking)")
			for _, d := range policyResult.Denials {
				fmt.Printf("   • %s\n", d)
			}
		}
		if len(policyResult.Warnings) > 0 {
			fmt.Println("\n⚠️  POLICY WARNINGS")
			for _, w := range policyResult.Warnings {
				fmt.Printf("   • %s\n", w)
			}
		}
		if len(policyResult.Denials) == 0 && len(policyResult.Warnings) == 0 {
			fmt.Println("\n✅ All policies passed")
		}
	}

	fmt.Println()
}
