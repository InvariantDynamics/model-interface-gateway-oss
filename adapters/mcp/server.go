package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Adapter struct {
	manifest Manifest
	baseURL  string
	client   *http.Client
	token    string
	tenantID string
}

func NewAdapter(baseURL string, manifest Manifest) *Adapter {
	return &Adapter{
		manifest: manifest,
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (a *Adapter) SetBearerToken(token string) {
	a.token = strings.TrimSpace(token)
}

func (a *Adapter) SetTenantID(tenantID string) {
	a.tenantID = strings.TrimSpace(tenantID)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type helloRequest struct {
	Header            map[string]interface{} `json:"header"`
	SupportedVersions []string               `json:"supported_versions"`
	RequestedBindings []string               `json:"requested_bindings"`
	RequestedFeatures []string               `json:"requested_features"`
}

type discoverRequest struct {
	Header map[string]interface{} `json:"header"`
}

type invokeRequest struct {
	Header           map[string]interface{} `json:"header"`
	Capability       string                 `json:"capability"`
	Payload          map[string]interface{} `json:"payload"`
	StreamPreference string                 `json:"stream_preference"`
}

func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRPC(w, http.StatusMethodNotAllowed, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32600, Message: "POST required"}})
		return
	}
	defer r.Body.Close()
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPC(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	var resp rpcResponse
	switch req.Method {
	case "initialize":
		result, err := a.handleInitialize(req)
		resp = buildRPCResponse(req, result, err)
	case "tools/list":
		result, err := a.handleToolsList(req)
		resp = buildRPCResponse(req, result, err)
	case "tools/call":
		result, err := a.handleToolsCall(req)
		resp = buildRPCResponse(req, result, err)
	default:
		resp = rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}
	}
	writeRPC(w, http.StatusOK, resp)
}

func buildRPCResponse(req rpcRequest, result interface{}, err *rpcError) rpcResponse {
	if err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: err}
	}
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (a *Adapter) handleInitialize(_ rpcRequest) (interface{}, *rpcError) {
	hello := helloRequest{
		Header: map[string]interface{}{
			"mig_version": "0.1",
			"tenant_id":   "mcp-adapter",
		},
		SupportedVersions: []string{"0.1"},
		RequestedBindings: []string{a.manifest.DefaultBinding},
		RequestedFeatures: []string{"mcp-bridge"},
	}
	var migResp map[string]interface{}
	if err := a.postJSON("/mig/v0.1/hello", hello, &migResp); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "mcp-mig-adapter",
			"version": "0.1.0",
		},
		"mig": migResp,
	}, nil
}

func (a *Adapter) handleToolsList(_ rpcRequest) (interface{}, *rpcError) {
	discover := discoverRequest{Header: map[string]interface{}{"mig_version": "0.1", "tenant_id": "mcp-adapter"}}
	var response struct {
		Capabilities []struct {
			ID string `json:"id"`
		} `json:"capabilities"`
	}
	if err := a.postJSON("/mig/v0.1/discover", discover, &response); err != nil {
		return nil, err
	}
	tools := make([]map[string]interface{}, 0, len(a.manifest.Mappings.Tools))
	for _, mapping := range a.manifest.Mappings.Tools {
		tools = append(tools, map[string]interface{}{
			"name":        mapping.MCPName,
			"description": fmt.Sprintf("MIG capability %s", mapping.MIGCapability),
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		})
	}
	return map[string]interface{}{"tools": tools}, nil
}

func (a *Adapter) handleToolsCall(req rpcRequest) (interface{}, *rpcError) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params"}
	}
	mapping, ok := a.manifest.CapabilityForTool(params.Name)
	if !ok {
		return nil, &rpcError{Code: -32601, Message: "tool not mapped"}
	}
	invoke := invokeRequest{
		Header: map[string]interface{}{
			"mig_version": "0.1",
			"tenant_id":   "mcp-adapter",
			"meta": map[string]interface{}{
				"idg.product_tier": "core",
				"idg.meter_key":    "invocations_capability_tier",
			},
		},
		Capability: mapping.MIGCapability,
		Payload:    params.Arguments,
	}
	var resp struct {
		Payload map[string]interface{} `json:"payload"`
	}
	if err := a.postJSON("/mig/v0.1/invoke/"+mapping.MIGCapability, invoke, &resp); err != nil {
		return nil, err
	}
	resultBytes, _ := json.Marshal(resp.Payload)
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(resultBytes)},
		},
		"isError": false,
	}, nil
}

func (a *Adapter) postJSON(path string, reqBody interface{}, dst interface{}) *rpcError {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return &rpcError{Code: -32603, Message: "marshal request failed", Data: err.Error()}
	}
	httpReq, err := http.NewRequest(http.MethodPost, a.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return &rpcError{Code: -32603, Message: "create request failed", Data: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.token)
	}
	if a.tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", a.tenantID)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return &rpcError{Code: -32011, Message: "backend unavailable", Data: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &rpcError{Code: -32603, Message: "mig backend error", Data: string(body)}
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return &rpcError{Code: -32603, Message: "decode backend response failed", Data: err.Error()}
	}
	return nil
}

func writeRPC(w http.ResponseWriter, status int, resp rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
