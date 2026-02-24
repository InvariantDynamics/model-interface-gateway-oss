package mig

import (
	"context"
	"testing"
	"time"
)

func TestHeaderNormalizeDefaults(t *testing.T) {
	head := MessageHeader{TenantID: "acme"}
	if err := head.Normalize(time.Now()); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if head.MIGVersion != MIGVersion {
		t.Fatalf("unexpected version: %s", head.MIGVersion)
	}
	if head.MessageID == "" {
		t.Fatal("expected generated message id")
	}
	if head.DeadlineMS == 0 {
		t.Fatal("expected default deadline")
	}
}

func TestInvokeIdempotency(t *testing.T) {
	svc := NewService()
	req := InvokeRequest{
		Header: MessageHeader{TenantID: "acme", IdempotencyKey: "id-1"},
		Payload: map[string]interface{}{
			"input": "hello",
		},
	}
	principal := Principal{
		Subject:       "tester",
		TenantID:      "acme",
		Scopes:        map[string]struct{}{"capability:infer": {}},
		Authenticated: true,
	}
	first, err := svc.Invoke(context.Background(), "observatory.models.infer", req, "tester", principal)
	if err != nil {
		t.Fatalf("invoke failed: %v", err.Message)
	}
	second, err := svc.Invoke(context.Background(), "observatory.models.infer", req, "tester", principal)
	if err != nil {
		t.Fatalf("invoke failed: %v", err.Message)
	}
	if first.Header.MessageID == "" || second.Header.MessageID == "" {
		t.Fatal("message ids must be present")
	}
	if first.Payload["result"] != second.Payload["result"] {
		t.Fatalf("expected idempotent payloads to match: %#v vs %#v", first.Payload, second.Payload)
	}
}
