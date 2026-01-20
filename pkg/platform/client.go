package platform

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
	"log/slog"
	"fmt"
)

type HTTPClient struct {
	Client  *http.Client
	Retries int
	Timeout time.Duration
	Logger  *slog.Logger
}

func NewHTTPClient(retries int, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		Client: &http.Client{
			Timeout: timeout,
		},
		Retries: retries,
		Timeout: timeout,
		Logger:  slog.Default(),
	}
}

func (c *HTTPClient) PostJSON(url string, body []byte) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= c.Retries; i++ {
		req, rErr := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if rErr != nil {
			return nil, rErr
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.Client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			// Success or client error (do not retry 4xx usually, unless throttling)
			return resp, nil
		}

		if i < c.Retries {
			c.Logger.Warn("HTTP request failed, retrying", "url", url, "attempt", i+1, "error", err)
			time.Sleep(time.Duration(1<<i) * 200 * time.Millisecond) // Exponential backoff
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.Retries, err)
	}
	return resp, nil // return last response even if 500
}
