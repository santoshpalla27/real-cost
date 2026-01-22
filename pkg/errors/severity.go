// Package errors provides severity-aware error types.
package errors

import "fmt"

// Severity indicates error impact level.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
	SeverityFatal
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// FiacError is a structured error with context.
type FiacError struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Severity    Severity `json:"severity"`
	ResourceID  string   `json:"resource_id,omitempty"`
	Recoverable bool     `json:"recoverable"`
}

func (e *FiacError) Error() string {
	if e.ResourceID != "" {
		return fmt.Sprintf("[%s] %s: %s (resource: %s)", e.Severity, e.Code, e.Message, e.ResourceID)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message)
}

// Error codes
const (
	ErrCodeParseFailed      = "PARSE_FAILED"
	ErrCodeUnknownResource  = "UNKNOWN_RESOURCE"
	ErrCodeUnsupportedType  = "UNSUPPORTED_TYPE"
	ErrCodeMissingAttribute = "MISSING_ATTRIBUTE"
	ErrCodePriceNotFound    = "PRICE_NOT_FOUND"
	ErrCodeLowConfidence    = "LOW_CONFIDENCE"
	ErrCodePolicyViolation  = "POLICY_VIOLATION"
)

// NewUnknownResourceError creates an error for unmapped resources.
func NewUnknownResourceError(resourceType, resourceID string) *FiacError {
	return &FiacError{
		Code:        ErrCodeUnknownResource,
		Message:     fmt.Sprintf("No semantic mapping for resource type: %s", resourceType),
		Severity:    SeverityError,
		ResourceID:  resourceID,
		Recoverable: false,
	}
}

// NewMissingAttributeError creates an error for missing required attributes.
func NewMissingAttributeError(attribute, resourceID string) *FiacError {
	return &FiacError{
		Code:        ErrCodeMissingAttribute,
		Message:     fmt.Sprintf("Missing required attribute: %s", attribute),
		Severity:    SeverityError,
		ResourceID:  resourceID,
		Recoverable: false,
	}
}

// NewPriceNotFoundError creates an error for unresolved pricing.
func NewPriceNotFoundError(sku, resourceID string) *FiacError {
	return &FiacError{
		Code:        ErrCodePriceNotFound,
		Message:     fmt.Sprintf("Price not found for SKU: %s", sku),
		Severity:    SeverityError,
		ResourceID:  resourceID,
		Recoverable: false,
	}
}
