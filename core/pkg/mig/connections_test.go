package mig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionsRegisterAndFilter(t *testing.T) {
	svc := NewService()

	_, unregisterA := svc.RegisterConnection(ConnectionSnapshot{Protocol: "http", Kind: "ws_stream", TenantID: "acme", Actor: "alice"})
	_, unregisterB := svc.RegisterConnection(ConnectionSnapshot{Protocol: "grpc", Kind: "stream_invoke", TenantID: "beta", Actor: "bob"})
	defer unregisterA()
	defer unregisterB()

	all := svc.Connections(ConnectionFilters{})
	if all.Summary.Total != 2 {
		t.Fatalf("expected 2 connections, got %d", all.Summary.Total)
	}

	acme := svc.Connections(ConnectionFilters{TenantID: "acme"})
	if acme.Summary.Total != 1 {
		t.Fatalf("expected tenant filter to return 1, got %d", acme.Summary.Total)
	}
	if len(acme.Connections) != 1 || acme.Connections[0].TenantID != "acme" {
		t.Fatalf("unexpected tenant filter response: %#v", acme.Connections)
	}

	unregisterA()
	after := svc.Connections(ConnectionFilters{})
	if after.Summary.Total != 1 {
		t.Fatalf("expected 1 connection after unregister, got %d", after.Summary.Total)
	}
}

func TestAdminConnectionsAndUIEndpoints(t *testing.T) {
	svc := NewService()
	_, unregister := svc.RegisterConnection(ConnectionSnapshot{Protocol: "http", Kind: "sse_subscribe", TenantID: "acme", Actor: "alice"})
	defer unregister()

	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, svc)
	server := httptest.NewServer(AuthMiddleware(AuthConfig{Mode: AuthModeNone})(mux))
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/v0.1/connections?tenant_id=acme")
	if err != nil {
		t.Fatalf("connections request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connections status: %d", resp.StatusCode)
	}
	var payload ConnectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Summary.Total != 1 {
		t.Fatalf("expected 1 active connection, got %d", payload.Summary.Total)
	}

	uiResp, err := http.Get(server.URL + "/ui")
	if err != nil {
		t.Fatalf("ui request failed: %v", err)
	}
	defer uiResp.Body.Close()
	if uiResp.StatusCode != http.StatusOK {
		t.Fatalf("ui status: %d", uiResp.StatusCode)
	}
	buf := make([]byte, 64)
	n, _ := uiResp.Body.Read(buf)
	if n == 0 {
		t.Fatal("ui body is empty")
	}
}
