// Package main provides the unified FIAC API Server.
// This is the main production server that exposes all estimation endpoints.
package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/santoshpalla27/fiac-platform/internal/estimation"
	"github.com/santoshpalla27/fiac-platform/internal/graph"
	"github.com/santoshpalla27/fiac-platform/internal/policy"
	"github.com/santoshpalla27/fiac-platform/internal/pricing"
	"github.com/santoshpalla27/fiac-platform/internal/semantics"
	"github.com/santoshpalla27/fiac-platform/internal/usage"
	"github.com/santoshpalla27/fiac-platform/pkg/api"
)

var (
	version   = "0.1.0"
	startTime = time.Now()
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("ENV") == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	policiesDir := os.Getenv("POLICIES_DIR")
	if policiesDir == "" {
		policiesDir = "policies"
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health endpoints (for ALB/NLB)
	r.Get("/health", handleHealth)
	r.Get("/health/live", handleLiveness)
	r.Get("/health/ready", handleReadiness)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/estimate", handleEstimate(policiesDir))
		r.Post("/parse", handleParse)
		r.Post("/semantics", handleSemantics)
		r.Post("/usage", handleUsage)
		r.Post("/pricing", handlePricing)
	})

	// Metadata
	r.Get("/version", handleVersion)

	log.Info().
		Str("port", port).
		Str("version", version).
		Str("policies_dir", policiesDir).
		Msg("Starting FIAC API Server")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// Health check handlers for load balancer
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "healthy",
		"service": "fiac-api",
		"version": version,
		"uptime":  time.Since(startTime).String(),
	})
}

func handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Check if policies directory is accessible
	policiesDir := os.Getenv("POLICIES_DIR")
	if policiesDir == "" {
		policiesDir = "policies"
	}
	if _, err := os.Stat(policiesDir); os.IsNotExist(err) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not ready",
			"reason": "policies directory not found",
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("READY"))
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": version,
		"service": "fiac-api",
	})
}

// EstimateRequest is the full estimation request body
type EstimateRequest struct {
	PlanJSON    json.RawMessage `json:"plan_json"`
	Environment string          `json:"environment"`
}

// EstimateResponse is the full estimation response
type EstimateResponse struct {
	Estimation *estimation.Result `json:"estimation"`
	Policy     *policy.Result     `json:"policy,omitempty"`
	Success    bool               `json:"success"`
	Error      string             `json:"error,omitempty"`
}

func handleEstimate(policiesDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req EstimateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if len(req.PlanJSON) == 0 {
			respondError(w, http.StatusBadRequest, "plan_json is required")
			return
		}

		env := req.Environment
		if env == "" {
			env = "dev"
		}

		// Step 1: Parse plan
		infraGraph, err := graph.ParseTerraformPlan(req.PlanJSON)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "failed to parse terraform plan: "+err.Error())
			return
		}

		// Step 2: Extract semantics
		semanticEngine := semantics.NewEngine()
		billingResult, err := semanticEngine.Process(infraGraph)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "semantic processing failed: "+err.Error())
			return
		}

		// Step 3: Predict usage
		predictor := usage.NewPredictor()
		usageResult := predictor.Predict(billingResult.Components, env)

		if usageResult.UnknownEnvironment {
			respondError(w, http.StatusBadRequest, usageResult.EnvironmentError)
			return
		}

		// Step 4: Resolve pricing
		resolver := pricing.NewResolver()
		priceResult, err := resolver.Resolve(billingResult.Components, infraGraph.ProviderContext.Region, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "pricing resolution failed: "+err.Error())
			return
		}

		// Step 5: Calculate estimation
		calculator := estimation.NewCalculator()
		estimateResult, err := calculator.CalculateFromComponents(billingResult, usageResult, priceResult)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "estimation failed: "+err.Error())
			return
		}

		// Step 6: Evaluate policies
		evaluator := policy.NewEvaluator(policiesDir)
		policyResult, err := evaluator.Evaluate(estimateResult)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "policy evaluation failed: "+err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(EstimateResponse{
			Estimation: estimateResult,
			Policy:     policyResult,
			Success:    true,
		})
	}
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	var req api.ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := graph.ParseTerraformPlan(req.PlanJSON)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleSemantics(w http.ResponseWriter, r *http.Request) {
	var req api.SemanticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	engine := semantics.NewEngine()
	result, err := engine.Process(req.Graph)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleUsage(w http.ResponseWriter, r *http.Request) {
	var req api.UsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	predictor := usage.NewPredictor()
	result := predictor.Predict(req.Components, req.Environment)

	if result.UnknownEnvironment {
		respondError(w, http.StatusBadRequest, result.EnvironmentError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handlePricing(w http.ResponseWriter, r *http.Request) {
	var req api.PriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resolver := pricing.NewResolver()
	result, err := resolver.Resolve(req.Components, req.Region, req.EffectiveDate)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   message,
	})
}
