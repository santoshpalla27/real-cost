package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/futuristic-iac/estimation-core/service"
	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/platform"
)

func main() {
	// Initialize Shared Platform Components
	logger := platform.InitLogger()
	
	usageURL := platform.GetEnv("USAGE_URL", "http://localhost:8083/forecast")
	pricingURL := platform.GetEnv("PRICING_URL", "http://localhost:8084/price")
	
	estimator := service.NewEstimator(usageURL, pricingURL)

	http.HandleFunc("/estimate", platform.BasicAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var components []api.BillingComponent
		if err := json.Unmarshal(body, &components); err != nil {
			http.Error(w, "Invalid input JSON", http.StatusBadRequest)
			return
		}

		result, err := estimator.Estimate(components)
		if err != nil {
			logger.Error("Estimation failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := platform.GetEnv("PORT", ":8085")
	logger.Info("Estimation Core starting", "port", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		platform.LogFatal(logger, "Server failed", err)
	}
}
