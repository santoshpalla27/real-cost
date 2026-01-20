package service

import (
	"encoding/json"
	"os"
	"time"

	"github.com/futuristic-iac/pkg/api"
)

type AuditLogger struct {
	LogFile string
}

type AuditEntry struct {
	Timestamp      time.Time             `json:"timestamp"`
	PolicyInput    *api.EstimationResult `json:"input"`
	PolicyDecision *api.PolicyResult     `json:"decision"`
	Allowed        bool                  `json:"allowed"`
	Hash           string                `json:"hash"` // simulated content hash
}

func NewAuditLogger(file string) *AuditLogger {
	return &AuditLogger{LogFile: file}
}

func (l *AuditLogger) Log(input *api.EstimationResult, decision *api.PolicyResult) error {
	entry := AuditEntry{
		Timestamp:      time.Now(),
		PolicyInput:    input,
		PolicyDecision: decision,
		Allowed:        decision.Allowed,
		Hash:           "simulated-hash-sha256", 
	}
	
	f, err := os.OpenFile(l.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return err
	}
	
	return nil
}
