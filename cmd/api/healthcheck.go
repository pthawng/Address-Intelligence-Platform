package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func checkHealth() error {
	endpoint := os.Getenv("HEALTHCHECK_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8080/health/live"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("health request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health request returned status %d", response.StatusCode)
	}
	return nil
}
