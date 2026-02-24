package mig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Service struct {
	mu sync.RWMutex

	serverID string
	metrics  *Metrics

	capabilities map[string]CapabilityDescriptor
	schemas      map[string]map[string]interface{}
	events       map[string][]EventMessage
	subscribers  map[string]map[chan EventMessage]struct{}
	idempotency  map[string]InvokeResponse
	cancelled    map[string]string
	quotas       map[string]int64
	audit        []AuditRecord
	connections  map[string]ConnectionSnapshot

	tenantInvocations     map[string]int64
	capabilityInvocations map[string]int64

	orgs     map[string]Org
	tenants  map[string]Tenant
	gateways map[string]Gateway

	natsBinding *NATSBinding
	natsConn    *nats.Conn
	auditLog    *os.File
}

type ServiceOptions struct {
	NATSURL      string
	AuditLogPath string
}

func NewService() *Service {
	svc, err := NewServiceWithOptions(ServiceOptions{})
	if err != nil {
		panic(err)
	}
	return svc
}

func NewServiceWithOptions(opts ServiceOptions) (*Service, error) {
	s := &Service{
		serverID:              "migd-core",
		capabilities:          map[string]CapabilityDescriptor{},
		schemas:               map[string]map[string]interface{}{},
		events:                map[string][]EventMessage{},
		subscribers:           map[string]map[chan EventMessage]struct{}{},
		idempotency:           map[string]InvokeResponse{},
		cancelled:             map[string]string{},
		quotas:                map[string]int64{},
		tenantInvocations:     map[string]int64{},
		capabilityInvocations: map[string]int64{},
		connections:           map[string]ConnectionSnapshot{},
		orgs:                  map[string]Org{},
		tenants:               map[string]Tenant{},
		gateways:              map[string]Gateway{},
	}
	if opts.NATSURL != "" {
		nc, err := nats.Connect(opts.NATSURL)
		if err != nil {
			return nil, fmt.Errorf("connect nats: %w", err)
		}
		s.natsConn = nc
	}
	if opts.AuditLogPath != "" {
		file, err := os.OpenFile(opts.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open audit log: %w", err)
		}
		s.auditLog = file
	}
	s.bootstrapDefaults()
	return s, nil
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.natsBinding != nil {
		s.natsBinding.Close()
		s.natsBinding = nil
	}
	if s.natsConn != nil {
		s.natsConn.Close()
		s.natsConn = nil
	}
	if s.auditLog != nil {
		_ = s.auditLog.Close()
		s.auditLog = nil
	}
}

func (s *Service) SetMetrics(metrics *Metrics) {
	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()
}

func (s *Service) recordError(code, operation string) {
	s.mu.RLock()
	metrics := s.metrics
	s.mu.RUnlock()
	if metrics != nil {
		metrics.RecordError(code, operation)
	}
}

func (s *Service) bootstrapDefaults() {
	s.capabilities["observatory.models.infer"] = CapabilityDescriptor{
		ID:              "observatory.models.infer",
		Version:         "1.0.0",
		Modes:           []string{"unary", "server_stream"},
		InputSchemaURI:  "schema://observatory/models/infer-input/v1",
		OutputSchemaURI: "schema://observatory/models/infer-output/v1",
		EventTopics:     []string{"observatory.inference.completed"},
		AuthScopes:      []string{"capability:infer"},
		QoS: QoSProfile{
			MaxPayloadBytes:   1024 * 1024,
			SupportsReplay:    true,
			DeliverySemantics: "at_least_once",
			SupportsOrdering:  true,
		},
	}
	s.schemas["schema://observatory/models/infer-input/v1"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"input": map[string]interface{}{"type": "string"},
		},
		"required": []string{"input"},
	}
	s.schemas["schema://observatory/models/infer-output/v1"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{"type": "string"},
		},
		"required": []string{"result"},
	}
}

func (s *Service) Hello(req HelloRequest) (HelloResponse, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		return HelloResponse{}, invalid(err.Error())
	}
	if len(req.SupportedVersions) == 0 {
		req.SupportedVersions = []string{MIGVersion}
	}
	compatible := false
	for _, v := range req.SupportedVersions {
		if strings.TrimSpace(v) == MIGVersion {
			compatible = true
			break
		}
	}
	if !compatible {
		return HelloResponse{}, &MigError{Code: ErrorVersionMismatch, Message: "no compatible MIG version", Retryable: false}
	}
	selectedBinding := "http"
	for _, b := range req.RequestedBindings {
		candidate := strings.ToLower(strings.TrimSpace(b))
		switch candidate {
		case "grpc", "nats", "http":
			selectedBinding = candidate
			goto selected
		}
	}
selected:
	head.AddIDGMeta("core")
	return HelloResponse{
		Header:          head,
		SelectedVersion: MIGVersion,
		SelectedBinding: selectedBinding,
		EnabledFeatures: req.RequestedFeatures,
		ServerID:        s.serverID,
	}, nil
}

func (s *Service) Discover(req DiscoverRequest, principal Principal) (DiscoverResponse, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		s.recordError(ErrorInvalidRequest, "discover")
		return DiscoverResponse{}, invalid(err.Error())
	}
	head.AddIDGMeta("core")

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]CapabilityDescriptor, 0, len(s.capabilities))
	for _, capDesc := range s.capabilities {
		if req.Query != "" && !strings.Contains(capDesc.ID, req.Query) {
			continue
		}
		if !principal.HasAnyScope(capDesc.AuthScopes) {
			continue
		}
		out = append(out, capDesc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return DiscoverResponse{Header: head, Capabilities: out}, nil
}

func (s *Service) Invoke(ctx context.Context, capability string, req InvokeRequest, actor string, principal Principal) (InvokeResponse, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		s.recordError(ErrorInvalidRequest, "invoke")
		return InvokeResponse{}, invalid(err.Error())
	}
	head.AddIDGMeta("core")
	if capability == "" {
		capability = req.Capability
	}
	if capability == "" {
		s.recordError(ErrorInvalidRequest, "invoke")
		return InvokeResponse{}, invalid("capability is required")
	}
	req.Capability = capability

	s.mu.RLock()
	capDesc, ok := s.capabilities[capability]
	if !ok {
		s.mu.RUnlock()
		s.recordError(ErrorUnsupportedCapability, "invoke")
		return InvokeResponse{}, &MigError{Code: ErrorUnsupportedCapability, Message: "capability not found", Retryable: false}
	}
	if !principal.HasAnyScope(capDesc.AuthScopes) {
		s.mu.RUnlock()
		s.recordError(ErrorForbidden, "invoke")
		return InvokeResponse{}, &MigError{Code: ErrorForbidden, Message: "insufficient capability scope", Retryable: false}
	}
	if reason, cancelled := s.cancelled[head.MessageID]; cancelled {
		s.mu.RUnlock()
		s.recordError(ErrorTimeout, "invoke")
		return InvokeResponse{}, &MigError{Code: ErrorTimeout, Message: "invocation cancelled: " + reason, Retryable: true}
	}
	if head.IdempotencyKey != "" {
		idKey := fmt.Sprintf("%s:%s:%s", head.TenantID, capability, head.IdempotencyKey)
		if cached, exists := s.idempotency[idKey]; exists {
			s.mu.RUnlock()
			cached.Header = head
			return cached, nil
		}
	}
	quota, hasQuota := s.quotas[head.TenantID]
	used := s.tenantInvocations[head.TenantID]
	s.mu.RUnlock()

	if hasQuota && used >= quota {
		s.recordError(ErrorRateLimited, "invoke")
		return InvokeResponse{}, &MigError{Code: ErrorRateLimited, Message: "tenant quota exceeded", Retryable: true}
	}

	deadline := time.Duration(head.DeadlineMS) * time.Millisecond
	reqCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	type result struct {
		payload map[string]interface{}
		err     *MigError
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{payload: map[string]interface{}{
			"result":       "ok",
			"echo":         req.Payload,
			"capability":   capability,
			"delivery_qos": capDesc.QoS.DeliverySemantics,
		}}
	}()

	select {
	case <-reqCtx.Done():
		s.recordError(ErrorTimeout, "invoke")
		return InvokeResponse{}, &MigError{Code: ErrorTimeout, Message: "deadline exceeded", Retryable: true}
	case out := <-ch:
		if out.err != nil {
			s.recordError(out.err.Code, "invoke")
			return InvokeResponse{}, out.err
		}
		resp := InvokeResponse{
			Header:          head,
			Capability:      capability,
			Payload:         out.payload,
			ResultSchemaURI: capDesc.OutputSchemaURI,
		}
		s.mu.Lock()
		if head.IdempotencyKey != "" {
			idKey := fmt.Sprintf("%s:%s:%s", head.TenantID, capability, head.IdempotencyKey)
			s.idempotency[idKey] = resp
		}
		s.tenantInvocations[head.TenantID]++
		s.capabilityInvocations[capability]++
		s.audit = append(s.audit, AuditRecord{
			Actor:      actor,
			TenantID:   head.TenantID,
			Capability: capability,
			Outcome:    "success",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			MessageID:  head.MessageID,
		})
		s.writeAuditLogLocked(s.audit[len(s.audit)-1])
		s.mu.Unlock()
		return resp, nil
	}
}

func (s *Service) Publish(topic string, req PublishRequest) (PublishAck, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		s.recordError(ErrorInvalidRequest, "publish")
		return PublishAck{}, invalid(err.Error())
	}
	head.AddIDGMeta("core")
	if topic == "" {
		topic = req.Topic
	}
	if topic == "" {
		s.recordError(ErrorInvalidRequest, "publish")
		return PublishAck{}, invalid("topic is required")
	}
	if !strings.Contains(topic, ".") {
		s.recordError(ErrorInvalidRequest, "publish")
		return PublishAck{}, invalid("topic names must be namespaced")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sequence := int64(len(s.events[topic]) + 1)
	event := EventMessage{
		Header:      head,
		Topic:       topic,
		EventID:     newMessageID(),
		Sequence:    sequence,
		Payload:     req.Payload,
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
		Replay:      false,
	}
	s.events[topic] = append(s.events[topic], event)
	for sub := range s.subscribers[topic] {
		select {
		case sub <- event:
		default:
		}
	}
	s.publishEventToNATS(event)
	return PublishAck{
		Header:   head,
		Topic:    topic,
		EventID:  event.EventID,
		Sequence: sequence,
		Accepted: true,
	}, nil
}

func (s *Service) Subscribe(topic, resumeCursor string) ([]EventMessage, <-chan EventMessage, func(), *MigError) {
	if topic == "" {
		s.recordError(ErrorInvalidRequest, "subscribe")
		return nil, nil, nil, invalid("topic is required")
	}
	if !strings.Contains(topic, ".") {
		s.recordError(ErrorInvalidRequest, "subscribe")
		return nil, nil, nil, invalid("topic names must be namespaced")
	}
	start := 0
	if resumeCursor != "" {
		i, err := strconv.Atoi(resumeCursor)
		if err != nil || i < 0 {
			s.recordError(ErrorInvalidRequest, "subscribe")
			return nil, nil, nil, invalid("resume_cursor must be a non-negative integer")
		}
		start = i
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.events[topic]
	if start > len(all) {
		start = len(all)
	}
	snapshot := make([]EventMessage, len(all[start:]))
	copy(snapshot, all[start:])
	for i := range snapshot {
		snapshot[i].Replay = true
	}
	if s.subscribers[topic] == nil {
		s.subscribers[topic] = map[chan EventMessage]struct{}{}
	}
	ch := make(chan EventMessage, 32)
	s.subscribers[topic][ch] = struct{}{}
	unsub := func() {
		s.mu.Lock()
		if subs := s.subscribers[topic]; subs != nil {
			delete(subs, ch)
		}
		s.mu.Unlock()
		close(ch)
	}
	return snapshot, ch, unsub, nil
}

func (s *Service) Cancel(req CancelRequest, messageID string) (CancelAck, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		s.recordError(ErrorInvalidRequest, "cancel")
		return CancelAck{}, invalid(err.Error())
	}
	head.AddIDGMeta("core")
	if messageID == "" {
		messageID = req.TargetMessageID
	}
	if messageID == "" {
		s.recordError(ErrorInvalidRequest, "cancel")
		return CancelAck{}, invalid("target message id is required")
	}
	s.mu.Lock()
	s.cancelled[messageID] = req.Reason
	s.mu.Unlock()
	return CancelAck{
		Header:          head,
		TargetMessageID: messageID,
		Accepted:        true,
		Status:          "cancelled",
	}, nil
}

func (s *Service) Heartbeat(req HeartbeatRequest) (HeartbeatAck, *MigError) {
	head := req.Header
	if err := head.Normalize(time.Now()); err != nil {
		s.recordError(ErrorInvalidRequest, "heartbeat")
		return HeartbeatAck{}, invalid(err.Error())
	}
	head.AddIDGMeta("core")
	if req.IntervalMS <= 0 {
		req.IntervalMS = 5000
	}
	s.mu.RLock()
	load := float64(len(s.audit)) / 1000.0
	s.mu.RUnlock()
	return HeartbeatAck{Header: head, SuggestedIntervalMS: req.IntervalMS, LoadFactor: load}, nil
}

func (s *Service) AddCapability(req CapabilityUpsertRequest) *MigError {
	if req.Descriptor.ID == "" || req.Descriptor.Version == "" {
		return invalid("descriptor.id and descriptor.version are required")
	}
	if req.Descriptor.InputSchemaURI == "" || req.Descriptor.OutputSchemaURI == "" {
		return invalid("schema URIs are required")
	}
	s.mu.Lock()
	s.capabilities[req.Descriptor.ID] = req.Descriptor
	s.mu.Unlock()
	return nil
}

func (s *Service) ListCapabilities() []CapabilityDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CapabilityDescriptor, 0, len(s.capabilities))
	for _, desc := range s.capabilities {
		out = append(out, desc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) AddSchema(req SchemaUpsertRequest) *MigError {
	if req.URI == "" {
		return invalid("uri is required")
	}
	if len(req.Schema) == 0 {
		return invalid("schema is required")
	}
	s.mu.Lock()
	s.schemas[req.URI] = req.Schema
	s.mu.Unlock()
	return nil
}

func (s *Service) ConformanceHealth() ConformanceHealth {
	return ConformanceHealth{Core: true, Streaming: true, Evented: true, Full: true}
}

func (s *Service) ValidatePolicy(req PolicyValidateRequest) (PolicyValidateResponse, *MigError) {
	if req.TenantID == "" || req.Capability == "" || req.Action == "" {
		return PolicyValidateResponse{}, invalid("tenant_id, capability, and action are required")
	}
	if req.Action != "invoke" {
		return PolicyValidateResponse{Allowed: false, Reason: "unsupported action"}, nil
	}
	s.mu.RLock()
	_, exists := s.capabilities[req.Capability]
	s.mu.RUnlock()
	if !exists {
		return PolicyValidateResponse{Allowed: false, Reason: "capability does not exist"}, nil
	}
	return PolicyValidateResponse{Allowed: true}, nil
}

func (s *Service) SetQuota(req QuotaRequest) (QuotaResponse, *MigError) {
	if req.TenantID == "" {
		return QuotaResponse{}, invalid("tenant_id is required")
	}
	if req.MaxInvocations <= 0 {
		return QuotaResponse{}, invalid("max_invocations must be > 0")
	}
	s.mu.Lock()
	s.quotas[req.TenantID] = req.MaxInvocations
	s.mu.Unlock()
	return QuotaResponse{TenantID: req.TenantID, MaxInvocations: req.MaxInvocations}, nil
}

func (s *Service) AuditExport(tenantID string) []AuditRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if tenantID == "" {
		out := make([]AuditRecord, len(s.audit))
		copy(out, s.audit)
		return out
	}
	out := make([]AuditRecord, 0)
	for _, record := range s.audit {
		if record.TenantID == tenantID {
			out = append(out, record)
		}
	}
	return out
}

func (s *Service) CreateOrg(org Org) (Org, *MigError) {
	if org.Name == "" {
		return Org{}, invalid("name is required")
	}
	if org.ID == "" {
		org.ID = "org-" + newMessageID()[:12]
	}
	s.mu.Lock()
	s.orgs[org.ID] = org
	s.mu.Unlock()
	return org, nil
}

func (s *Service) CreateTenant(tenant Tenant) (Tenant, *MigError) {
	if tenant.Name == "" || tenant.OrgID == "" {
		return Tenant{}, invalid("name and org_id are required")
	}
	if tenant.ID == "" {
		tenant.ID = "tenant-" + newMessageID()[:12]
	}
	s.mu.Lock()
	if _, ok := s.orgs[tenant.OrgID]; !ok {
		s.mu.Unlock()
		return Tenant{}, &MigError{Code: ErrorNotFound, Message: "org not found", Retryable: false}
	}
	s.tenants[tenant.ID] = tenant
	s.mu.Unlock()
	return tenant, nil
}

func (s *Service) CreateGateway(gw Gateway) (Gateway, *MigError) {
	if gw.TenantID == "" || gw.Region == "" || gw.Binding == "" {
		return Gateway{}, invalid("tenant_id, region, and binding are required")
	}
	if gw.ID == "" {
		gw.ID = "gw-" + newMessageID()[:12]
	}
	s.mu.Lock()
	if _, ok := s.tenants[gw.TenantID]; !ok {
		s.mu.Unlock()
		return Gateway{}, &MigError{Code: ErrorNotFound, Message: "tenant not found", Retryable: false}
	}
	s.gateways[gw.ID] = gw
	s.mu.Unlock()
	return gw, nil
}

func (s *Service) Usage() UsageSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tenant := make(map[string]int64, len(s.tenantInvocations))
	for k, v := range s.tenantInvocations {
		tenant[k] = v
	}
	capability := make(map[string]int64, len(s.capabilityInvocations))
	var total int64
	for k, v := range s.capabilityInvocations {
		capability[k] = v
		total += v
	}
	return UsageSnapshot{TenantInvocations: tenant, CapabilityInvocations: capability, TotalInvocations: total}
}

func (s *Service) publishEventToNATS(event EventMessage) {
	if s.natsConn == nil {
		return
	}
	subject := fmt.Sprintf("mig.v0_1.%s.events.%s", sanitizeNATSSegment(event.Header.TenantID), sanitizeNATSSubject(event.Topic))
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	_ = s.natsConn.Publish(subject, body)
}

func sanitizeNATSSubject(value string) string {
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "/", ".")
	return value
}

func sanitizeNATSSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return sanitizeNATSSubject(value)
}

func (s *Service) writeAuditLogLocked(record AuditRecord) {
	if s.auditLog == nil {
		return
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	_, _ = s.auditLog.Write(append(payload, '\n'))
}

func invalid(msg string) *MigError {
	return &MigError{Code: ErrorInvalidRequest, Message: msg, Retryable: false}
}
