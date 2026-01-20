# Futuristic IaC - Cost Intelligence Platform

**Status**: [Industry-Grade MVP](https://github.com/futuristic-iac/cost)  
**Philosophy**: Fail-Closed | Audit-First | Policy-Driven

This platform provides Shift-Left Financial Governance for cloud infrastructure. It intercepts Terraform plans, calculates financial and carbon impact using specific policy rules, and enforcing budget constraints before deployment.

---

## 🏗 Architecture

The system follows a strict **Microservices DAG** architecture:

1.  **Ingestion Service** (:8081): Parses Terraform Plan JSON (`configuration` aware).
2.  **Semantic Engine** (:8082): Maps technical resources (`aws_instance`) to billable intents (Compute + Storage).
3.  **Usage Engine** (:8083): Predicts usage metrics (`hours`, `gb_months`) based on heuristics/history.
4.  **Pricing Engine** (:8084): FOCUS-compliant pricing lookup with Time-Travel semantics.
5.  **Estimation Core** (:8085): Orchestrates the Cost Calculation DAG (Usage -> Price -> Cost).
6.  **Policy Engine** (:8086): Enforces OPA policies and logs decisions to an Audit Trail.
7.  **Budget Service** (:8087): Provides budget thresholds for governance.

---

## 🚦 Production Readiness: The "Real" vs "Mock" List

This system is architecturally correct but contains mocks for external dependencies to facilitate standalone testing. **Review this list before deploying.**

### 1. Pricing Engine (`pricing-engine`)
*   **⚠️ Mocked:** The `ClickHouseStore` does **not** connect to a real ClickHouse database. It generates valid SQL and returns stubbed data ("t3.medium" @ $0.0416).
*   **Production Action:**
    *   Deploy a real ClickHouse cluster.
    *   Uncomment the `drive.Open` code in `storage/clickhouse.go`.
    *   Ingest real AWS Price List API data into the DB.

### 2. Usage Engine (`usage-engine`)
*   **⚠️ Mocked:** The `Predict` function uses static heuristics (e.g., "730 hours" for EC2, "1 month" for EBS). It does NOT query Prometheus or CloudWatch.
*   **Production Action:**
    *   Integrate with a Metrics Store (Prometheus/Thanos).
    *   Implement lookups map resource tags (`app-id`) to historical utilization.

### 3. Budget Service (`budget-service`)
*   **⚠️ Mocked:** Returns a hardcoded budget of `$1200` for every request.
*   **Production Action:**
    *   Connect to a real Financial planning system (Anaplan, Netsuite, or internally managed DB).
    *   Implement logic to fetch budgets by `CostCenter` or `ProjectID`.

### 4. Authentication (`pkg/platform/auth.go`)
*   **⚠️ Mocked:** The system uses Basic Auth. In this MVP, it reads `AUTH_USER`/`AUTH_PASS` from env, but previous defaults were `admin/admin`.
*   **Production Action:**
    *   **MUST** configure `AUTH_USER` and `AUTH_PASS` in the environment.
    *   Ideally, replace Basic Auth with **mTLS** or **OIDC/JWT** (Service-to-Service auth) for Kubernetes deployments.

### 5. Dependency Resolution (`ingestion-service`)
*   **✅ Implementation:** Uses explicit `depends_on` from Terraform Configuration.
*   **Limit:** Does not yet infer implicit dependencies (e.g., reference ID matching) fully.

---

## 🛠 How to Run

### Prerequisites
*   Go 1.23+
*   Docker & Docker Compose (for OPA/Redis/Prometheus)

### 1. Start Platform
This will launch all 7 microservices + ClickHouse + OPA.
```powershell
docker-compose up --build
```

### 2. Run Analysis (CLI)
The services are exposed on localhost ports (8081-8087).
```bash
# Generate Plan
terraform init
terraform plan -out=tfplan
terraform show -json tfplan > tfplan.json

# Analyze
cd fiac-cli
# Set Auth to match docker-compose environment
export AUTH_USER=admin
export AUTH_PASS=securepass123
go run main.go --plan ../tfplan.json
```

---

## ✅ Safety Validation (Phase 4 & 6)
To verify the **Fail-Closed** safety mechanisms:
1.  **Semantic Failure**: Modify a `resource_type` to something unknown. -> System returns `INCOMPLETE` (0 Confidence).
2.  **Pricing Failure**: Use an instance type with no price. -> System returns `INCOMPLETE`.
3.  **Policy Block**: If `Confidence < 0.8` or `Incomplete == true`, the Policy Engine **REJECTS** the plan.

---

## 📜 Audit Log
Policy decisions are written to `policy_audit.log` in the root directory.
Format: `JSON Lines` containing `{timestamp, input_hash, decision, violations}`.
