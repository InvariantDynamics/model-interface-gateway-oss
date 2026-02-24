package mig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func RegisterHTTPRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /mig/v0.1/hello", svc.handleHello)
	mux.HandleFunc("POST /mig/v0.1/discover", svc.handleDiscover)
	mux.HandleFunc("POST /mig/v0.1/invoke/{capability}", svc.handleInvoke)
	mux.HandleFunc("POST /mig/v0.1/publish/{topic}", svc.handlePublish)
	mux.HandleFunc("GET /mig/v0.1/subscribe/{topic}", svc.handleSubscribe)
	mux.HandleFunc("POST /mig/v0.1/cancel/{message_id}", svc.handleCancel)
	mux.HandleFunc("POST /mig/v0.1/heartbeat", svc.handleHeartbeat)
	mux.HandleFunc("GET /mig/v0.1/stream", svc.handleStream)

	mux.HandleFunc("POST /admin/v0.1/capabilities", svc.handleAddCapability)
	mux.HandleFunc("GET /admin/v0.1/capabilities", svc.handleListCapabilities)
	mux.HandleFunc("POST /admin/v0.1/schemas", svc.handleAddSchema)
	mux.HandleFunc("GET /admin/v0.1/health/conformance", svc.handleConformanceHealth)
	mux.HandleFunc("GET /admin/v0.1/connections", svc.handleConnections)

	mux.HandleFunc("GET /ui", svc.handleUI)

	mux.HandleFunc("POST /pro/v0.1/policies/validate", svc.handlePolicyValidate)
	mux.HandleFunc("POST /pro/v0.1/quotas", svc.handleSetQuota)
	mux.HandleFunc("GET /pro/v0.1/audit/export", svc.handleAuditExport)

	mux.HandleFunc("POST /cloud/v0.1/orgs", svc.handleCreateOrg)
	mux.HandleFunc("POST /cloud/v0.1/tenants", svc.handleCreateTenant)
	mux.HandleFunc("POST /cloud/v0.1/gateways", svc.handleCreateGateway)
	mux.HandleFunc("GET /cloud/v0.1/usage", svc.handleUsage)
}

func (s *Service) handleHello(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var req HelloRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	resp, err := s.Hello(req)
	if err != nil {
		writeMigError(w, req.Header, http.StatusBadRequest, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleDiscover(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var req DiscoverRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	resp, err := s.Discover(req, principal)
	if err != nil {
		writeMigError(w, req.Header, http.StatusBadRequest, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleInvoke(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	capability := r.PathValue("capability")
	var req InvokeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	actor := principal.Subject
	if actor == "" {
		actor = r.Header.Get("X-Actor")
		if actor == "" {
			actor = "anonymous"
		}
	}
	resp, err := s.Invoke(r.Context(), capability, req, actor, principal)
	if err != nil {
		status := http.StatusBadRequest
		if err.Code == ErrorUnsupportedCapability {
			status = http.StatusNotFound
		} else if err.Code == ErrorTimeout {
			status = http.StatusGatewayTimeout
		} else if err.Code == ErrorRateLimited {
			status = http.StatusTooManyRequests
		} else if err.Code == ErrorForbidden {
			status = http.StatusForbidden
		}
		writeMigError(w, req.Header, status, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handlePublish(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	topic := r.PathValue("topic")
	var req PublishRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	resp, err := s.Publish(topic, req)
	if err != nil {
		writeMigError(w, req.Header, http.StatusBadRequest, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.metrics != nil {
		s.metrics.IncActiveStream("sse")
		defer s.metrics.DecActiveStream("sse")
	}
	principal := principalFromContext(r.Context())
	topic := r.PathValue("topic")
	resumeCursor := r.URL.Query().Get("resume_cursor")
	events, stream, unsubscribe, err := s.Subscribe(topic, resumeCursor)
	if err != nil {
		writeMigError(w, MessageHeader{TenantID: tenantFromRequest(r)}, http.StatusBadRequest, *err)
		return
	}
	defer unsubscribe()

	tenantID := tenantFromRequest(r)
	if principal.TenantID != "" {
		tenantID = principal.TenantID
	}
	_, unregisterConn := s.RegisterConnection(ConnectionSnapshot{
		Protocol:   "http",
		Kind:       "sse_subscribe",
		TenantID:   tenantID,
		Actor:      principal.Subject,
		RemoteAddr: r.RemoteAddr,
		Meta: map[string]interface{}{
			"topic":         topic,
			"resume_cursor": resumeCursor,
		},
	})
	defer unregisterConn()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	sendEvent := func(event EventMessage) bool {
		buf, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return false
		}
		if _, writeErr := fmt.Fprintf(w, "event: mig-event\ndata: %s\n\n", buf); writeErr != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	for _, event := range events {
		if !sendEvent(event) {
			return
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if !sendEvent(event) {
				return
			}
		}
	}
}

func (s *Service) handleCancel(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	messageID := r.PathValue("message_id")
	var req CancelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	resp, err := s.Cancel(req, messageID)
	if err != nil {
		writeMigError(w, req.Header, http.StatusBadRequest, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var req HeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if migErr := applyPrincipalHeader(&req.Header, principal, r); migErr != nil {
		writeMigError(w, req.Header, http.StatusForbidden, *migErr)
		return
	}
	resp, err := s.Heartbeat(req)
	if err != nil {
		writeMigError(w, req.Header, http.StatusBadRequest, *err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

var streamUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func (s *Service) handleStream(w http.ResponseWriter, r *http.Request) {
	if s.metrics != nil {
		s.metrics.IncActiveStream("websocket")
		defer s.metrics.DecActiveStream("websocket")
	}
	principal := principalFromContext(r.Context())
	conn, err := streamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	tenantID := tenantFromRequest(r)
	if principal.TenantID != "" {
		tenantID = principal.TenantID
	}
	_, unregisterConn := s.RegisterConnection(ConnectionSnapshot{
		Protocol:   "http",
		Kind:       "ws_stream",
		TenantID:   tenantID,
		Actor:      principal.Subject,
		RemoteAddr: r.RemoteAddr,
		Meta:       map[string]interface{}{"path": r.URL.Path},
	})
	defer unregisterConn()

	for {
		var frame StreamFrame
		if readErr := conn.ReadJSON(&frame); readErr != nil {
			return
		}
		if frame.StreamID == "" {
			frame.StreamID = "stream-" + frame.Header.MessageID
		}
		if migErr := applyPrincipalHeader(&frame.Header, principal, r); migErr != nil {
			_ = conn.WriteJSON(StreamFrame{
				Header:     frame.Header,
				StreamID:   frame.StreamID,
				Capability: frame.Capability,
				Kind:       "error",
				EndStream:  true,
				Error:      migErr,
			})
			continue
		}

		switch frame.Kind {
		case "request":
			invokeReq := InvokeRequest{
				Header:     frame.Header,
				Capability: frame.Capability,
				Payload:    frame.Payload,
			}
			actor := principal.Subject
			if actor == "" {
				actor = "anonymous"
			}
			resp, invokeErr := s.Invoke(context.Background(), frame.Capability, invokeReq, actor, principal)
			out := StreamFrame{
				Header:     frame.Header,
				StreamID:   frame.StreamID,
				Capability: frame.Capability,
				Kind:       "response",
				EndStream:  true,
			}
			if invokeErr != nil {
				out.Kind = "error"
				out.Error = invokeErr
			} else {
				out.Payload = resp.Payload
			}
			if writeErr := conn.WriteJSON(out); writeErr != nil {
				return
			}
		case "control":
			action := ""
			if frame.Payload != nil {
				if raw, ok := frame.Payload["action"].(string); ok {
					action = strings.ToLower(strings.TrimSpace(raw))
				}
			}
			if action != "cancel" {
				_ = conn.WriteJSON(StreamFrame{
					Header:    frame.Header,
					StreamID:  frame.StreamID,
					Kind:      "error",
					EndStream: true,
					Error: &MigError{
						Code:      ErrorInvalidRequest,
						Message:   "unsupported control action",
						Retryable: false,
					},
				})
				continue
			}
			cancelReq := CancelRequest{
				Header:          frame.Header,
				TargetMessageID: frame.Header.MessageID,
				Reason:          "websocket control cancel",
			}
			ack, cancelErr := s.Cancel(cancelReq, cancelReq.TargetMessageID)
			out := StreamFrame{
				Header:    frame.Header,
				StreamID:  frame.StreamID,
				Kind:      "control",
				EndStream: true,
			}
			if cancelErr != nil {
				out.Kind = "error"
				out.Error = cancelErr
			} else {
				out.Payload = map[string]interface{}{
					"accepted": ack.Accepted,
					"status":   ack.Status,
				}
			}
			if writeErr := conn.WriteJSON(out); writeErr != nil {
				return
			}
		default:
			_ = conn.WriteJSON(StreamFrame{
				Header:     frame.Header,
				StreamID:   frame.StreamID,
				Capability: frame.Capability,
				Kind:       "error",
				EndStream:  true,
				Error: &MigError{
					Code:      ErrorInvalidRequest,
					Message:   "frame.kind must be request or control",
					Retryable: false,
				},
			})
		}
	}
}

func (s *Service) handleAddCapability(w http.ResponseWriter, r *http.Request) {
	var req CapabilityUpsertRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.AddCapability(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusCreated, req.Descriptor)
}

func (s *Service) handleListCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"capabilities": s.ListCapabilities()})
}

func (s *Service) handleAddSchema(w http.ResponseWriter, r *http.Request) {
	var req SchemaUpsertRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.AddSchema(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"uri": req.URI})
}

func (s *Service) handleConformanceHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.ConformanceHealth())
}

func (s *Service) handleConnections(w http.ResponseWriter, r *http.Request) {
	filters := ConnectionFilters{
		TenantID: r.URL.Query().Get("tenant_id"),
		Kind:     r.URL.Query().Get("kind"),
		Protocol: r.URL.Query().Get("protocol"),
	}
	writeJSON(w, http.StatusOK, s.Connections(filters))
}

func (s *Service) handleUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(migConsoleHTML))
}

func (s *Service) handlePolicyValidate(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var req PolicyValidateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if principal.TenantID != "" && req.TenantID != principal.TenantID {
		writeMigError(w, MessageHeader{TenantID: req.TenantID}, http.StatusForbidden, MigError{
			Code:      ErrorForbidden,
			Message:   "tenant_id does not match authenticated principal",
			Retryable: false,
		})
		return
	}
	resp, err := s.ValidatePolicy(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleSetQuota(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var req QuotaRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if principal.TenantID != "" && req.TenantID != principal.TenantID {
		writeMigError(w, MessageHeader{TenantID: req.TenantID}, http.StatusForbidden, MigError{
			Code:      ErrorForbidden,
			Message:   "tenant_id does not match authenticated principal",
			Retryable: false,
		})
		return
	}
	resp, err := s.SetQuota(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	writeJSON(w, http.StatusOK, map[string]interface{}{"records": s.AuditExport(tenantID)})
}

func (s *Service) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var org Org
	if !decodeJSON(w, r, &org) {
		return
	}
	resp, err := s.CreateOrg(org)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var tenant Tenant
	if !decodeJSON(w, r, &tenant) {
		return
	}
	resp, err := s.CreateTenant(tenant)
	if err != nil {
		status := http.StatusBadRequest
		if err.Code == ErrorNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) handleCreateGateway(w http.ResponseWriter, r *http.Request) {
	var gw Gateway
	if !decodeJSON(w, r, &gw) {
		return
	}
	resp, err := s.CreateGateway(gw)
	if err != nil {
		status := http.StatusBadRequest
		if err.Code == ErrorNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Message})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Service) handleUsage(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Usage())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if r.Body == nil {
		http.Error(w, "request body required", http.StatusBadRequest)
		return false
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeMigError(w http.ResponseWriter, header MessageHeader, status int, migErr MigError) {
	if header.TenantID == "" {
		header.TenantID = "unknown"
	}
	if header.MessageID == "" {
		header.MessageID = newMessageID()
	}
	if header.MIGVersion == "" {
		header.MIGVersion = MIGVersion
	}
	if header.Timestamp == "" {
		header.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	writeJSON(w, status, ErrorEnvelope{Header: header, Error: migErr})
}

func tenantFromRequest(r *http.Request) string {
	tenant := r.Header.Get("X-Tenant-ID")
	if tenant != "" {
		return strings.TrimSpace(tenant)
	}
	return "unknown"
}
