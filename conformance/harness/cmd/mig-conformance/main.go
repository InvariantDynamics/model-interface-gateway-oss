package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

type check struct {
	Name string
	Path string
	Body map[string]interface{}
}

func main() {
	baseURL := flag.String("base-url", "http://localhost:8080", "migd base URL")
	tenantID := flag.String("tenant", "acme", "tenant id used in checks")
	token := flag.String("token", "", "optional bearer token")
	flag.Parse()

	checks := []check{
		{
			Name: "hello",
			Path: "/mig/v0.1/hello",
			Body: map[string]interface{}{
				"header":             map[string]interface{}{"tenant_id": *tenantID},
				"supported_versions": []string{"0.1"},
				"requested_bindings": []string{"http"},
			},
		},
		{
			Name: "discover",
			Path: "/mig/v0.1/discover",
			Body: map[string]interface{}{
				"header": map[string]interface{}{"tenant_id": *tenantID},
			},
		},
		{
			Name: "invoke",
			Path: "/mig/v0.1/invoke/observatory.models.infer",
			Body: map[string]interface{}{
				"header":  map[string]interface{}{"tenant_id": *tenantID},
				"payload": map[string]interface{}{"input": "health-check"},
			},
		},
	}

	client := &http.Client{}
	failed := false
	for _, c := range checks {
		status, body, err := runCheck(client, *baseURL+c.Path, c.Body, *tenantID, *token)
		if err != nil {
			fmt.Printf("[FAIL] %s: %v\n", c.Name, err)
			failed = true
			continue
		}
		if status >= 400 {
			fmt.Printf("[FAIL] %s: status=%d body=%s\n", c.Name, status, body)
			failed = true
			continue
		}
		fmt.Printf("[PASS] %s\n", c.Name)
	}

	if failed {
		os.Exit(1)
	}
}

func runCheck(client *http.Client, url string, body map[string]interface{}, tenantID, token string) (int, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}
