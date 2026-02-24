package harness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/InvariantDynamics/model-interface-gateway-oss/core/pkg/mig"
)

func newTestServer() (*mig.Service, *httptest.Server) {
	svc := mig.NewService()
	mux := http.NewServeMux()
	mig.RegisterHTTPRoutes(mux, svc)
	authCfg := mig.AuthConfig{Mode: mig.AuthModeNone}
	return svc, httptest.NewServer(mig.AuthMiddleware(authCfg)(mux))
}

func TestHelloAndDiscover(t *testing.T) {
	_, ts := newTestServer()
	defer ts.Close()

	helloReq := mig.HelloRequest{
		Header:            mig.MessageHeader{TenantID: "acme"},
		SupportedVersions: []string{"0.1"},
		RequestedBindings: []string{"http"},
	}
	var helloResp mig.HelloResponse
	postJSON(t, ts.URL+"/mig/v0.1/hello", helloReq, &helloResp)
	if helloResp.SelectedVersion != "0.1" {
		t.Fatalf("unexpected selected version: %q", helloResp.SelectedVersion)
	}

	discoverReq := mig.DiscoverRequest{Header: mig.MessageHeader{TenantID: "acme"}}
	var discoverResp mig.DiscoverResponse
	postJSON(t, ts.URL+"/mig/v0.1/discover", discoverReq, &discoverResp)
	if len(discoverResp.Capabilities) == 0 {
		t.Fatal("expected at least one capability")
	}
}

func TestInvokeCancelAndQuota(t *testing.T) {
	_, ts := newTestServer()
	defer ts.Close()

	quotaReq := mig.QuotaRequest{TenantID: "acme", MaxInvocations: 1}
	var quotaResp mig.QuotaResponse
	postJSON(t, ts.URL+"/pro/v0.1/quotas", quotaReq, &quotaResp)
	if quotaResp.MaxInvocations != 1 {
		t.Fatalf("expected quota 1, got %d", quotaResp.MaxInvocations)
	}

	invokeReq := mig.InvokeRequest{
		Header:     mig.MessageHeader{TenantID: "acme", MessageID: "invoke-1"},
		Capability: "observatory.models.infer",
		Payload:    map[string]interface{}{"input": "hello"},
	}
	var invokeResp mig.InvokeResponse
	postJSON(t, ts.URL+"/mig/v0.1/invoke/observatory.models.infer", invokeReq, &invokeResp)
	if invokeResp.Capability != "observatory.models.infer" {
		t.Fatalf("unexpected capability: %s", invokeResp.Capability)
	}

	// Second invocation exceeds quota.
	status, body := postJSONRaw(t, ts.URL+"/mig/v0.1/invoke/observatory.models.infer", invokeReq)
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for quota exceeded, got %d: %s", status, string(body))
	}

	cancelReq := mig.CancelRequest{Header: mig.MessageHeader{TenantID: "acme"}, TargetMessageID: "invoke-99", Reason: "operator"}
	var cancelResp mig.CancelAck
	postJSON(t, ts.URL+"/mig/v0.1/cancel/invoke-99", cancelReq, &cancelResp)
	if !cancelResp.Accepted {
		t.Fatal("expected cancel accepted")
	}
}

func TestPublishSubscribeReplay(t *testing.T) {
	svc := mig.NewService()
	ack, err := svc.Publish("observatory.inference.completed", mig.PublishRequest{
		Header:  mig.MessageHeader{TenantID: "acme"},
		Payload: map[string]interface{}{"state": "done"},
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err.Message)
	}
	if ack.Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", ack.Sequence)
	}

	replay, stream, unsubscribe, err2 := svc.Subscribe("observatory.inference.completed", "0")
	if err2 != nil {
		t.Fatalf("subscribe failed: %v", err2.Message)
	}
	defer unsubscribe()
	if len(replay) != 1 || !replay[0].Replay {
		t.Fatalf("expected one replay event, got %#v", replay)
	}

	_, err3 := svc.Publish("observatory.inference.completed", mig.PublishRequest{
		Header:  mig.MessageHeader{TenantID: "acme"},
		Payload: map[string]interface{}{"state": "new"},
	})
	if err3 != nil {
		t.Fatalf("publish failed: %v", err3.Message)
	}

	select {
	case live := <-stream:
		if live.Replay {
			t.Fatal("live stream event must not be replay")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for live stream event")
	}
}

func TestWebSocketStreamInvoke(t *testing.T) {
	_, ts := newTestServer()
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/mig/v0.1/stream"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	frame := mig.StreamFrame{
		Header: mig.MessageHeader{
			TenantID:  "acme",
			MessageID: "stream-msg-1",
		},
		StreamID:   "stream-1",
		Capability: "observatory.models.infer",
		Kind:       "request",
		Payload:    map[string]interface{}{"input": "hello"},
	}
	if err := conn.WriteJSON(frame); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var response mig.StreamFrame
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read json: %v", err)
	}
	if response.Kind != "response" {
		t.Fatalf("expected response frame, got %s", response.Kind)
	}
	if !response.EndStream {
		t.Fatal("expected end_stream=true")
	}
	if response.Payload["result"] != "ok" {
		t.Fatalf("unexpected stream payload: %#v", response.Payload)
	}
}

func postJSON(t *testing.T, url string, req any, dst any) {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
}

func postJSONRaw(t *testing.T, url string, req any) (int, []byte) {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}
