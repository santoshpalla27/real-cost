package fiac

# Carbon policy - enforce sustainability limits

deny[msg] {
    input.carbon_kg > 1000
    msg := sprintf("Carbon footprint (%.1f kgCO2e) exceeds 1000 kg limit", [input.carbon_kg])
}

warn[msg] {
    input.carbon_kg > 500
    input.carbon_kg <= 1000
    msg := sprintf("Carbon footprint (%.1f kgCO2e) approaching limit", [input.carbon_kg])
}
