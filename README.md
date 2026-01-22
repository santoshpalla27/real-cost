# FIAC Platform - IaC Cost Intelligence

A shift-left financial and carbon control plane for Terraform infrastructure.

## Overview

FIAC Platform intercepts Terraform plans **before deployment** to:
- Predict realistic costs with uncertainty (P50/P90)
- Calculate carbon footprint
- Enforce governance policies
- Block or warn in CI/CD pipelines
- Explain every number and decision

## Architecture

```
┌──────────────────────────┐
│   Terraform Plan JSON    │
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│   IaC Ingestion Service  │  Parse → Infrastructure Graph
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Billing Semantic Engine │  Resources → Billing Components
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Predictive Usage Engine │  Heuristic forecasts with confidence
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Pricing & Carbon Engine │  SKU resolution + carbon intensity
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Cost Estimation Core    │  Cost DAG + confidence aggregation
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Policy Engine (OPA)     │  Budget, growth, carbon limits
└───────────┬──────────────┘
            ▼
┌──────────────────────────┐
│  Developer Feedback      │  CLI / PR comments / JSON for CI
└──────────────────────────┘
```

## Quick Start

### Prerequisites
- Docker & Docker Compose

### Deploy (One Command)

```bash
docker-compose up -d
```

That's it! The API server is now running at `http://localhost:8080`

### Verify

```bash
# Health check
curl http://localhost:8080/health

# Estimate costs
curl -X POST http://localhost:8080/api/v1/estimate \
  -H "Content-Type: application/json" \
  -d '{"plan_json": {...}, "environment": "prod"}'
```

### Stop

```bash
docker-compose down
```

### CLI Usage (Optional)

```bash
go run ./cmd/cli estimate --plan examples/tfplan.json --output json
```

## Project Structure

```
fiac-platform/
├── cmd/                    # Service entrypoints
│   ├── ingestion/          # IaC Ingestion Service
│   ├── semantic/           # Billing Semantic Engine
│   ├── usage/              # Predictive Usage Engine
│   ├── pricing/            # Pricing & Carbon Engine
│   ├── estimation/         # Cost Estimation Core
│   └── cli/                # Developer CLI
│
├── internal/               # Private domain logic
│   ├── graph/              # Infrastructure graph
│   ├── semantics/          # Billing semantics
│   ├── usage/              # Usage prediction
│   ├── pricing/            # Price resolution
│   ├── estimation/         # Cost calculation
│   ├── policy/             # OPA integration
│   └── carbon/             # Carbon modeling
│
├── pkg/                    # Shared contracts
│   ├── api/                # Request/response structs
│   ├── units/              # Canonical units
│   ├── errors/             # Severity-aware errors
│   └── confidence/         # Confidence math
│
├── policies/               # OPA Rego policies
├── deployments/            # Docker & Kubernetes
└── examples/               # Sample Terraform plans
```

## Design Principles

1. **Fail Closed** - Unknown resources generate errors, not silent skips
2. **Explainable** - Every number has a traceable origin
3. **Deterministic** - Same input always produces same output
4. **Container-First** - One binary per service, no runtime compilation
5. **CI/CD Safe** - Proper exit codes and machine-readable output

## License

MIT
