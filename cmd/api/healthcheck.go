package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func checkHealth(ctx context.Context) error {
	endpoint := os.Getenv("HEALTHCHECK_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8080/health/live"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("invalid health request")
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("health request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health request returned status %d", response.StatusCode)
	}
	return nil
}
