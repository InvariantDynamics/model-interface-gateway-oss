package mig

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketStreamInvoke(t *testing.T) {
	svc := NewService()
	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, svc)
	srv := httptest.NewServer(AuthMiddleware(AuthConfig{Mode: AuthModeNone})(mux))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/mig/v0.1/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	frame := StreamFrame{
		Header: MessageHeader{
			MIGVersion: "0.1",
			TenantID:   "acme",
			MessageID:  "msg-stream-1",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
		StreamID:   "stream-1",
		Capability: "observatory.models.infer",
		Kind:       "request",
		Payload: map[string]interface{}{
			"input": "hello",
		},
	}
	if err := conn.WriteJSON(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	var resp StreamFrame
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if resp.Kind != "response" {
		t.Fatalf("expected response frame, got %s (%+v)", resp.Kind, resp.Error)
	}
	if !resp.EndStream {
		t.Fatal("expected end_stream=true")
	}
	if resp.Payload["result"] != "ok" {
		t.Fatalf("unexpected payload: %#v", resp.Payload)
	}
}
