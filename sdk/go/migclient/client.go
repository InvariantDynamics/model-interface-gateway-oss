package migclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type MessageHeader struct {
	MIGVersion string `json:"mig_version"`
	MessageID  string `json:"message_id,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	TenantID   string `json:"tenant_id"`
}

type HelloRequest struct {
	Header            MessageHeader `json:"header"`
	SupportedVersions []string      `json:"supported_versions"`
	RequestedBindings []string      `json:"requested_bindings"`
}

type HelloResponse struct {
	SelectedVersion string `json:"selected_version"`
	SelectedBinding string `json:"selected_binding"`
}

type DiscoverRequest struct {
	Header MessageHeader `json:"header"`
	Query  string        `json:"query,omitempty"`
}

type Capability struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	Modes   []string `json:"modes"`
}

type DiscoverResponse struct {
	Capabilities []Capability `json:"capabilities"`
}

type InvokeRequest struct {
	Header     MessageHeader  `json:"header"`
	Capability string         `json:"capability,omitempty"`
	Payload    map[string]any `json:"payload"`
}

type InvokeResponse struct {
	Capability string         `json:"capability"`
	Payload    map[string]any `json:"payload"`
}

func (c *Client) Hello(ctx context.Context, req HelloRequest) (HelloResponse, error) {
	var resp HelloResponse
	if err := c.post(ctx, "/mig/v0.1/hello", req, &resp); err != nil {
		return HelloResponse{}, err
	}
	return resp, nil
}

func (c *Client) Discover(ctx context.Context, req DiscoverRequest) (DiscoverResponse, error) {
	var resp DiscoverResponse
	if err := c.post(ctx, "/mig/v0.1/discover", req, &resp); err != nil {
		return DiscoverResponse{}, err
	}
	return resp, nil
}

func (c *Client) Invoke(ctx context.Context, capability string, req InvokeRequest) (InvokeResponse, error) {
	var resp InvokeResponse
	if err := c.post(ctx, "/mig/v0.1/invoke/"+capability, req, &resp); err != nil {
		return InvokeResponse{}, err
	}
	return resp, nil
}

func (c *Client) post(ctx context.Context, path string, reqBody any, dst any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
