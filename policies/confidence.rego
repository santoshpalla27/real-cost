package fiac

# Confidence policy - warn on low confidence estimates

warn[msg] {
    input.confidence_score < 0.8
    msg := sprintf("Low confidence score (%.0f%%) - estimates may be unreliable", [input.confidence_score * 100])
}

deny[msg] {
    input.confidence_score < 0.5
    msg := sprintf("Confidence score (%.0f%%) too low for approval", [input.confidence_score * 100])
}

deny[msg] {
    input.is_incomplete == true
    msg := "Incomplete estimate - some resources could not be mapped"
}
