// Package main provides the Predictive Usage Engine entrypoint.
// This service predicts resource usage with uncertainty bounds.
package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/santoshpalla27/fiac-platform/internal/usage"
	"github.com/santoshpalla27/fiac-platform/pkg/api"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", handleHealth)
	r.Post("/api/v1/predict", handlePredict)

	log.Info().Str("port", port).Msg("Starting Predictive Usage Engine")
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "usage",
	})
}

func handlePredict(w http.ResponseWriter, r *http.Request) {
	var req api.UsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	predictor := usage.NewPredictor()
	result := predictor.Predict(req.Components, req.Environment)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
