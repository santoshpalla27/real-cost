// Package api provides the HTTP API server for TerraCost
// This is the main entry point for the Decision Plane services
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"terraform-cost/db/clickhouse"
	"terraform-cost/decision/billing"
	"terraform-cost/decision/billing/mappers/aws"
	"terraform-cost/decision/estimation"
	"terraform-cost/decision/iac"
	"terraform-cost/decision/policy"
)

// Server is the HTTP API server
type Server struct {
	httpServer    *http.Server
	pricingStore  *clickhouse.Store
	billingEngine *billing.Engine
	policyEngine  *policy.Engine
	config        *Config
}

// Config holds server configuration
type Config struct {
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxRequestSize int64
	CORSOrigins    []string
	OPAEndpoint    string
}

// DefaultConfig returns default server configuration
func DefaultConfig() *Config {
	return &Config{
		Port:           8080,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxRequestSize: 10 * 1024 * 1024, // 10MB
		CORSOrigins:    []string{"*"},
	}
}

// NewServer creates a new API server
func NewServer(store *clickhouse.Store, config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize billing engine with AWS mappers
	billingEngine := billing.NewEngine()
	aws.RegisterAllMappers(billingEngine)

	// Initialize policy engine
	policyEngine := policy.NewEngine()
	if config.OPAEndpoint != "" {
		policyEngine.WithOPA(config.OPAEndpoint)
	}

	return &Server{
		pricingStore:  store,
		billingEngine: billingEngine,
		policyEngine:  policyEngine,
		config:        config,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/api/v1/estimate", s.handleEstimate)
	mux.HandleFunc("/api/v1/estimate/", s.handleEstimate)
	mux.HandleFunc("/api/v1/policy/evaluate", s.handlePolicyEvaluate)
	mux.HandleFunc("/api/v1/snapshots", s.handleListSnapshots)

	// Wrap with middleware
	handler := s.corsMiddleware(s.loggingMiddleware(mux))

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      handler,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	fmt.Printf("🚀 TerraCost API server starting on port %d\n", s.config.Port)
	return s.httpServer.ListenAndServe()
}

// StartWithGracefulShutdown starts server with graceful shutdown handling
func (s *Server) StartWithGracefulShutdown() error {
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.Start(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		fmt.Println("\n📴 Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
}

// =============================================================================
// MIDDLEWARE
// =============================================================================

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %s %s\n", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		// Check if origin is allowed
		allowed := false
		for _, o := range s.config.CORSOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// =============================================================================
// HEALTH ENDPOINTS
// =============================================================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": "1.0.0",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check database connectivity
	if err := s.pricingStore.Ping(ctx); err != nil {
		s.jsonError(w, http.StatusServiceUnavailable, "database not ready")
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

// =============================================================================
// ESTIMATE ENDPOINT
// =============================================================================

// EstimateRequest is the API request for cost estimation
type EstimateRequest struct {
	Plan            json.RawMessage `json:"plan"`
	Environment     string          `json:"environment"`
	IncludeCarbon   bool            `json:"include_carbon"`
	IncludeFormulas bool            `json:"include_formulas"`
	CostLimit       *float64        `json:"cost_limit,omitempty"`
	CarbonBudget    *float64        `json:"carbon_budget,omitempty"`
}

// EstimateResponse is the API response for cost estimation
type EstimateResponse struct {
	// Cost metrics
	MonthlyCostP50 string  `json:"monthly_cost_p50"`
	MonthlyCostP90 string  `json:"monthly_cost_p90"`
	HourlyCostP50  string  `json:"hourly_cost_p50"`
	CarbonKgCO2    float64 `json:"carbon_kg_co2"`

	// Quality
	Confidence   float64 `json:"confidence"`
	IsIncomplete bool    `json:"is_incomplete"`

	// Statistics
	ResourceCount       int `json:"resource_count"`
	ComponentsEstimated int `json:"components_estimated"`
	ComponentsSymbolic  int `json:"components_symbolic"`

	// Policy
	PolicyResult string             `json:"policy_result"`
	Violations   []policy.Violation `json:"violations"`
	Warnings     []policy.Warning   `json:"warnings"`

	// Cost breakdown
	CostDrivers []CostDriverResponse `json:"cost_drivers"`

	// Audit
	EstimatedAt   string            `json:"estimated_at"`
	SnapshotsUsed map[string]string `json:"snapshots_used"`
}

// CostDriverResponse is a single cost line item
type CostDriverResponse struct {
	ID             string  `json:"id"`
	ResourceAddr   string  `json:"resource_addr"`
	Service        string  `json:"service"`
	ProductFamily  string  `json:"product_family"`
	Region         string  `json:"region"`
	Description    string  `json:"description"`
	MonthlyCostP50 string  `json:"monthly_cost_p50"`
	MonthlyCostP90 string  `json:"monthly_cost_p90"`
	Formula        string  `json:"formula,omitempty"`
	Confidence     float64 `json:"confidence"`
	IsSymbolic     bool    `json:"is_symbolic"`
	Reason         string  `json:"reason,omitempty"`
}

func (s *Server) handleEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request size
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxRequestSize)

	// Parse request
	var req EstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	ctx := r.Context()

	// Parse Terraform plan
	parser := iac.NewParser()
	plan, err := parser.ParseBytes(req.Plan)
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid terraform plan: %v", err))
		return
	}

	// Build infrastructure graph
	graphBuilder := iac.NewGraphBuilder()
	graph, err := graphBuilder.Build(plan)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to build graph: %v", err))
		return
	}

	// Decompose into billing components
	decomposition, err := s.billingEngine.Decompose(graph)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("billing decomposition failed: %v", err))
		return
	}

	// Run estimation
	estimationEngine := estimation.NewEngine(s.pricingStore)
	estResult, err := estimationEngine.Estimate(ctx, estimation.EstimationRequest{
		Components:      decomposition.Components,
		Environment:     req.Environment,
		IncludeCarbon:   req.IncludeCarbon,
		IncludeFormulas: req.IncludeFormulas,
	})
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("estimation failed: %v", err))
		return
	}

	// Run policy evaluation
	policyReq := policy.EvaluationRequest{
		Estimation:  estResult,
		Environment: req.Environment,
	}

	// Add custom policies from request
	if req.CostLimit != nil {
		policyReq.CustomPolicies = append(policyReq.CustomPolicies, policy.Policy{
			ID:        "api-cost-limit",
			Name:      "Cost Limit",
			Type:      policy.PolicyTypeCostLimit,
			Severity:  policy.SeverityError,
			Threshold: *req.CostLimit,
			Enabled:   true,
		})
	}

	if req.CarbonBudget != nil {
		policyReq.CustomPolicies = append(policyReq.CustomPolicies, policy.Policy{
			ID:        "api-carbon-budget",
			Name:      "Carbon Budget",
			Type:      policy.PolicyTypeCarbonBudget,
			Severity:  policy.SeverityError,
			Threshold: *req.CarbonBudget,
			Enabled:   true,
		})
	}

	policyResult, err := s.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		// Policy evaluation is non-fatal
		policyResult = &policy.EvaluationResult{
			Decision: policy.DecisionPass,
			Warnings: []policy.Warning{{Message: fmt.Sprintf("policy evaluation failed: %v", err)}},
		}
	}

	// Build response
	resp := s.buildEstimateResponse(estResult, policyResult, graph)
	s.jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) buildEstimateResponse(est *estimation.EstimationResult, pol *policy.EvaluationResult, graph *iac.Graph) EstimateResponse {
	// Convert cost drivers
	drivers := make([]CostDriverResponse, len(est.CostDrivers))
	for i, d := range est.CostDrivers {
		drivers[i] = CostDriverResponse{
			ID:             d.ID,
			ResourceAddr:   d.ResourceAddr,
			Service:        d.Service,
			ProductFamily:  d.ProductFamily,
			Region:         d.Region,
			Description:    d.Description,
			MonthlyCostP50: d.MonthlyCostP50.StringFixed(2),
			MonthlyCostP90: d.MonthlyCostP90.StringFixed(2),
			Formula:        d.Formula,
			Confidence:     d.Confidence,
			IsSymbolic:     d.IsSymbolic,
			Reason:         d.Reason,
		}
	}

	// Convert snapshot IDs
	snapshots := make(map[string]string)
	for region, id := range est.AuditTrail.SnapshotsUsed {
		snapshots[region] = id.String()
	}

	return EstimateResponse{
		MonthlyCostP50:      est.MonthlyCostP50.StringFixed(2),
		MonthlyCostP90:      est.MonthlyCostP90.StringFixed(2),
		HourlyCostP50:       est.HourlyCostP50.StringFixed(4),
		CarbonKgCO2:         est.CarbonKgCO2,
		Confidence:          est.Confidence,
		IsIncomplete:        est.IsIncomplete,
		ResourceCount:       graph.ResourceCount,
		ComponentsEstimated: est.ComponentsEstimated,
		ComponentsSymbolic:  est.ComponentsSymbolic,
		PolicyResult:        string(pol.Decision),
		Violations:          pol.Violations,
		Warnings:            pol.Warnings,
		CostDrivers:         drivers,
		EstimatedAt:         est.AuditTrail.EstimatedAt.Format(time.RFC3339),
		SnapshotsUsed:       snapshots,
	}
}

// =============================================================================
// POLICY ENDPOINT
// =============================================================================

func (s *Server) handlePolicyEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// This endpoint evaluates policies against an existing estimation
	// For now, redirect to /api/v1/estimate which includes policy evaluation
	s.jsonError(w, http.StatusNotImplemented, "use /api/v1/estimate for policy evaluation")
}

// =============================================================================
// SNAPSHOT ENDPOINT
// =============================================================================

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cloud := r.URL.Query().Get("cloud")
	region := r.URL.Query().Get("region")

	if cloud == "" {
		cloud = "aws"
	}
	if region == "" {
		region = "us-east-1"
	}

	ctx := r.Context()
	snapshots, err := s.pricingStore.ListSnapshots(ctx, clickhouse.CloudProvider(cloud), region)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list snapshots: %v", err))
		return
	}

	type SnapshotResponse struct {
		ID        string `json:"id"`
		Cloud     string `json:"cloud"`
		Region    string `json:"region"`
		Source    string `json:"source"`
		Hash      string `json:"hash"`
		IsActive  bool   `json:"is_active"`
		FetchedAt string `json:"fetched_at"`
		CreatedAt string `json:"created_at"`
	}

	resp := make([]SnapshotResponse, len(snapshots))
	for i, snap := range snapshots {
		resp[i] = SnapshotResponse{
			ID:        snap.ID.String(),
			Cloud:     string(snap.Cloud),
			Region:    snap.Region,
			Source:    snap.Source,
			Hash:      snap.Hash[:16] + "...",
			IsActive:  snap.IsActive,
			FetchedAt: snap.FetchedAt.Format(time.RFC3339),
			CreatedAt: snap.CreatedAt.Format(time.RFC3339),
		}
	}

	s.jsonResponse(w, http.StatusOK, resp)
}

// =============================================================================
// HELPERS
// =============================================================================

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]string{
		"error": message,
	})
}

// Unused but required for imports
var _ = uuid.Nil
var _ = decimal.Zero
var _ = io.EOF
