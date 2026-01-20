# Start all microservices in background jobs
Write-Host "Starting Services..."

Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "ingestion-service/main.go"
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "semantic-engine/main.go"
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "usage-engine/main.go"
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "pricing-engine/main.go"
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "estimation-core/main.go"
Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "policy-engine/main.go"

Write-Host "Services started! Waiting for health checks..."
Start-Sleep -Seconds 5
Write-Host "Ready to run CLI."
