// Package confidence provides confidence score math utilities.
package confidence

import "math"

// Aggregate combines multiple confidence scores.
// Uses geometric mean to penalize low-confidence components.
func Aggregate(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}

	product := 1.0
	for _, s := range scores {
		if s <= 0 {
			return 0
		}
		product *= s
	}

	return math.Pow(product, 1.0/float64(len(scores)))
}

// Decay applies uncertainty decay to a base confidence.
// Each factor reduces confidence proportionally.
func Decay(base float64, factors int) float64 {
	if factors <= 0 {
		return base
	}
	// Each factor reduces confidence by 10%
	decayRate := 0.9
	return base * math.Pow(decayRate, float64(factors))
}

// AboveThreshold checks if confidence meets minimum requirement.
func AboveThreshold(score, threshold float64) bool {
	return score >= threshold
}

// WeightedAverage calculates weighted confidence.
func WeightedAverage(scores []float64, weights []float64) float64 {
	if len(scores) == 0 || len(scores) != len(weights) {
		return 0
	}

	var sum, weightSum float64
	for i, s := range scores {
		sum += s * weights[i]
		weightSum += weights[i]
	}

	if weightSum == 0 {
		return 0
	}
	return sum / weightSum
}

// Clamp ensures confidence is in valid range [0, 1].
func Clamp(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// DefaultConfidence values
const (
	HighConfidence   = 0.95
	MediumConfidence = 0.80
	LowConfidence    = 0.60
	MinConfidence    = 0.50
)
