package platform

import (
	"os"
	"strconv"
	"strings"
)

// Config helper to read env vars with defaults
type ConfigLoader struct{}

func GetEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

func GetEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func GetEnvBool(key string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(key); exists {
		if strings.ToLower(val) == "true" || val == "1" {
			return true
		}
		return false
	}
	return defaultVal
}
