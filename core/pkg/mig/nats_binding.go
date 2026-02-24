package mig

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSBinding struct {
	nc   *nats.Conn
	svc  *Service
	subs []*nats.Subscription
}

func (s *Service) StartNATSBinding() (*NATSBinding, error) {
	s.mu.Lock()
	if s.natsBinding != nil {
		binding := s.natsBinding
		s.mu.Unlock()
		return binding, nil
	}
	nc := s.natsConn
	s.mu.Unlock()
	if nc == nil {
		return nil, fmt.Errorf("nats connection is not configured")
	}
	binding := &NATSBinding{nc: nc, svc: s}

	subjectHandlers := map[string]func(*nats.Msg){
		"mig.v0_1.*.hello":             binding.handleHello,
		"mig.v0_1.*.discover":          binding.handleDiscover,
		"mig.v0_1.*.invoke.>":          binding.handleInvoke,
		"mig.v0_1.*.events.>":          binding.handlePublish,
		"mig.v0_1.*.control.cancel.>":  binding.handleCancel,
		"mig.v0_1.*.control.heartbeat": binding.handleHeartbeat,
	}

	for subject, handler := range subjectHandlers {
		sub, err := nc.Subscribe(subject, handler)
		if err != nil {
			binding.Close()
			return nil, fmt.Errorf("subscribe %s: %w", subject, err)
		}
		binding.subs = append(binding.subs, sub)
	}
	s.mu.Lock()
	s.natsBinding = binding
	s.mu.Unlock()
	return binding, nil
}

func (b *NATSBinding) Close() {
	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	b.subs = nil
}

func (b *NATSBinding) handleHello(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	var req HelloRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid hello request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	resp, err := b.svc.Hello(req)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func (b *NATSBinding) handleDiscover(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	var req DiscoverRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid discover request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	principal := Principal{TenantID: req.Header.TenantID, Scopes: map[string]struct{}{}, Authenticated: false}
	resp, err := b.svc.Discover(req, principal)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func (b *NATSBinding) handleInvoke(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	capability := strings.Join(subjectTokens(msg.Subject)[4:], ".")
	var req InvokeRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid invoke request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	if req.Capability == "" {
		req.Capability = capability
	}
	principal := Principal{TenantID: req.Header.TenantID, Scopes: map[string]struct{}{}, Authenticated: false}
	resp, err := b.svc.Invoke(context.Background(), req.Capability, req, "nats-client", principal)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func (b *NATSBinding) handlePublish(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	topic := strings.Join(subjectTokens(msg.Subject)[4:], ".")
	var req PublishRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid publish request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	if req.Topic == "" {
		req.Topic = topic
	}
	resp, err := b.svc.Publish(req.Topic, req)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func (b *NATSBinding) handleCancel(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	messageID := strings.Join(subjectTokens(msg.Subject)[5:], ".")
	var req CancelRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid cancel request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	if req.TargetMessageID == "" {
		req.TargetMessageID = messageID
	}
	resp, err := b.svc.Cancel(req, req.TargetMessageID)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func (b *NATSBinding) handleHeartbeat(msg *nats.Msg) {
	tenant := subjectToken(msg.Subject, 2)
	var req HeartbeatRequest
	if !decodeNATS(msg, &req) {
		respondNATSError(msg, ErrorInvalidRequest, "invalid heartbeat request")
		return
	}
	if req.Header.TenantID == "" {
		req.Header.TenantID = tenant
	}
	resp, err := b.svc.Heartbeat(req)
	if err != nil {
		respondNATSMigError(msg, req.Header, *err)
		return
	}
	respondNATS(msg, resp)
}

func decodeNATS(msg *nats.Msg, dst interface{}) bool {
	if len(msg.Data) == 0 {
		return false
	}
	return json.Unmarshal(msg.Data, dst) == nil
}

func respondNATS(msg *nats.Msg, payload interface{}) {
	if msg.Reply == "" {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = msg.Respond(body)
}

func respondNATSError(msg *nats.Msg, code, message string) {
	if msg.Reply == "" {
		return
	}
	_ = msg.Respond([]byte(fmt.Sprintf(`{"error":{"code":"%s","message":%q}}`, code, message)))
}

func respondNATSMigError(msg *nats.Msg, header MessageHeader, migErr MigError) {
	if msg.Reply == "" {
		return
	}
	head := header
	if head.TenantID == "" {
		head.TenantID = subjectToken(msg.Subject, 2)
	}
	if head.MessageID == "" {
		head.MessageID = newMessageID()
	}
	if head.MIGVersion == "" {
		head.MIGVersion = MIGVersion
	}
	if head.Timestamp == "" {
		head.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	response := ErrorEnvelope{Header: head, Error: migErr}
	respondNATS(msg, response)
}

func subjectTokens(subject string) []string {
	if subject == "" {
		return nil
	}
	return strings.Split(subject, ".")
}

func subjectToken(subject string, index int) string {
	tokens := subjectTokens(subject)
	if index < 0 || index >= len(tokens) {
		return ""
	}
	return tokens[index]
}
