package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mig-standard/mig/core/pkg/mig"
)

func TestToolsListAndCall(t *testing.T) {
	svc := mig.NewService()
	mux := http.NewServeMux()
	mig.RegisterHTTPRoutes(mux, svc)
	backend := httptest.NewServer(mux)
	defer backend.Close()

	adapter := NewAdapter(backend.URL, Manifest{
		Version:        "0.2",
		SchemaVersion:  "0.2",
		DefaultBinding: "http",
		Mappings: MappingGroup{Tools: []ToolMapping{{
			MCPName:       "observatory_infer",
			MIGCapability: "observatory.models.infer",
			Mode:          "unary",
		}}},
	})
	adapterServer := httptest.NewServer(adapter)
	defer adapterServer.Close()

	listReq := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]interface{}{}}
	listStatus, listBody := postRPC(t, adapterServer.URL, listReq)
	if listStatus != http.StatusOK {
		t.Fatalf("tools/list failed: %d %s", listStatus, string(listBody))
	}
	if !bytes.Contains(listBody, []byte("observatory_infer")) {
		t.Fatalf("tools/list response missing mapped tool: %s", string(listBody))
	}

	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "observatory_infer",
			"arguments": map[string]interface{}{"input": "hello"},
		},
	}
	callStatus, callBody := postRPC(t, adapterServer.URL, callReq)
	if callStatus != http.StatusOK {
		t.Fatalf("tools/call failed: %d %s", callStatus, string(callBody))
	}
	if !bytes.Contains(callBody, []byte("result")) {
		t.Fatalf("tools/call missing invoke result: %s", string(callBody))
	}
}

func postRPC(t *testing.T, url string, payload map[string]interface{}) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http post failed: %v", err)
	}
	defer resp.Body.Close()
	respBody := new(bytes.Buffer)
	_, _ = respBody.ReadFrom(resp.Body)
	return resp.StatusCode, respBody.Bytes()
}
