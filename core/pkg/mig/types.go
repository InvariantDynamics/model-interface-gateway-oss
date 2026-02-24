package mig

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	MIGVersion = "0.1"

	ErrorInvalidRequest        = "MIG_INVALID_REQUEST"
	ErrorUnauthorized          = "MIG_UNAUTHORIZED"
	ErrorForbidden             = "MIG_FORBIDDEN"
	ErrorNotFound              = "MIG_NOT_FOUND"
	ErrorUnsupportedCapability = "MIG_UNSUPPORTED_CAPABILITY"
	ErrorVersionMismatch       = "MIG_VERSION_MISMATCH"
	ErrorTimeout               = "MIG_TIMEOUT"
	ErrorRateLimited           = "MIG_RATE_LIMITED"
	ErrorBackpressure          = "MIG_BACKPRESSURE"
	ErrorUnavailable           = "MIG_UNAVAILABLE"
	ErrorInternal              = "MIG_INTERNAL"
)

// MessageHeader is the canonical MIG v0.1 message envelope header.
type MessageHeader struct {
	MIGVersion     string                 `json:"mig_version"`
	MessageID      string                 `json:"message_id"`
	Timestamp      string                 `json:"timestamp"`
	TenantID       string                 `json:"tenant_id"`
	SessionID      string                 `json:"session_id,omitempty"`
	Traceparent    string                 `json:"traceparent,omitempty"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	DeadlineMS     int                    `json:"deadline_ms,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// Normalize applies MIG defaults and required field validation constraints.
func (h *MessageHeader) Normalize(now time.Time) error {
	if h == nil {
		return fmt.Errorf("header is required")
	}
	if h.MIGVersion == "" {
		h.MIGVersion = MIGVersion
	}
	if h.MIGVersion != MIGVersion {
		return fmt.Errorf("unsupported mig_version %q", h.MIGVersion)
	}
	if h.MessageID == "" {
		h.MessageID = newMessageID()
	}
	if h.Timestamp == "" {
		h.Timestamp = now.UTC().Format(time.RFC3339)
	}
	if h.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if h.DeadlineMS <= 0 {
		h.DeadlineMS = 30000
	}
	if h.Meta == nil {
		h.Meta = map[string]interface{}{}
	}
	return nil
}

// AddIDGMeta annotates standardized product telemetry fields in header meta.
func (h *MessageHeader) AddIDGMeta(productTier string) {
	if h.Meta == nil {
		h.Meta = map[string]interface{}{}
	}
	h.Meta["idg.product_tier"] = productTier
	h.Meta["idg.meter_key"] = "invocations_capability_tier"
}

func newMessageID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

type MigError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Retryable bool                   `json:"retryable"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Header MessageHeader `json:"header"`
	Error  MigError      `json:"error"`
}

type HelloRequest struct {
	Header            MessageHeader `json:"header"`
	SupportedVersions []string      `json:"supported_versions"`
	RequestedBindings []string      `json:"requested_bindings"`
	RequestedFeatures []string      `json:"requested_features,omitempty"`
}

type HelloResponse struct {
	Header          MessageHeader `json:"header"`
	SelectedVersion string        `json:"selected_version"`
	SelectedBinding string        `json:"selected_binding"`
	EnabledFeatures []string      `json:"enabled_features,omitempty"`
	ServerID        string        `json:"server_id,omitempty"`
}

type DiscoverRequest struct {
	Header            MessageHeader `json:"header"`
	Query             string        `json:"query,omitempty"`
	IncludeSchemaRefs bool          `json:"include_schema_refs,omitempty"`
	IncludeQoS        bool          `json:"include_qos,omitempty"`
}

type DiscoverResponse struct {
	Header       MessageHeader          `json:"header"`
	Capabilities []CapabilityDescriptor `json:"capabilities"`
}

type QoSProfile struct {
	MaxPayloadBytes   int64  `json:"max_payload_bytes,omitempty"`
	SupportsReplay    bool   `json:"supports_replay,omitempty"`
	DeliverySemantics string `json:"delivery_semantics,omitempty"`
	SupportsOrdering  bool   `json:"supports_ordering,omitempty"`
}

type CapabilityDescriptor struct {
	ID              string     `json:"id"`
	Version         string     `json:"version"`
	Modes           []string   `json:"modes"`
	InputSchemaURI  string     `json:"input_schema_uri"`
	OutputSchemaURI string     `json:"output_schema_uri"`
	EventTopics     []string   `json:"event_topics,omitempty"`
	AuthScopes      []string   `json:"auth_scopes"`
	QoS             QoSProfile `json:"qos,omitempty"`
}

type InvokeRequest struct {
	Header           MessageHeader          `json:"header"`
	Capability       string                 `json:"capability,omitempty"`
	Payload          map[string]interface{} `json:"payload"`
	StreamPreference string                 `json:"stream_preference,omitempty"`
}

type InvokeResponse struct {
	Header          MessageHeader          `json:"header"`
	Capability      string                 `json:"capability"`
	Payload         map[string]interface{} `json:"payload"`
	ResultSchemaURI string                 `json:"result_schema_uri,omitempty"`
}

type PublishRequest struct {
	Header  MessageHeader          `json:"header"`
	Topic   string                 `json:"topic,omitempty"`
	Key     string                 `json:"key,omitempty"`
	Payload map[string]interface{} `json:"payload"`
}

type PublishAck struct {
	Header   MessageHeader `json:"header"`
	Topic    string        `json:"topic"`
	EventID  string        `json:"event_id"`
	Sequence int64         `json:"sequence"`
	Accepted bool          `json:"accepted"`
}

type EventMessage struct {
	Header      MessageHeader          `json:"header"`
	Topic       string                 `json:"topic"`
	EventID     string                 `json:"event_id"`
	Sequence    int64                  `json:"sequence"`
	Payload     map[string]interface{} `json:"payload"`
	PublishedAt string                 `json:"published_at"`
	Replay      bool                   `json:"replay"`
}

type CancelRequest struct {
	Header          MessageHeader `json:"header"`
	TargetMessageID string        `json:"target_message_id,omitempty"`
	Reason          string        `json:"reason,omitempty"`
}

type CancelAck struct {
	Header          MessageHeader `json:"header"`
	TargetMessageID string        `json:"target_message_id"`
	Accepted        bool          `json:"accepted"`
	Status          string        `json:"status"`
}

type HeartbeatRequest struct {
	Header     MessageHeader `json:"header"`
	IntervalMS int           `json:"interval_ms,omitempty"`
}

type HeartbeatAck struct {
	Header              MessageHeader `json:"header"`
	SuggestedIntervalMS int           `json:"suggested_interval_ms"`
	LoadFactor          float64       `json:"load_factor"`
}

type StreamFrame struct {
	Header     MessageHeader          `json:"header"`
	StreamID   string                 `json:"stream_id"`
	Capability string                 `json:"capability"`
	Kind       string                 `json:"kind"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	EndStream  bool                   `json:"end_stream,omitempty"`
	Error      *MigError              `json:"error,omitempty"`
}

type ConformanceHealth struct {
	Core      bool `json:"core"`
	Streaming bool `json:"streaming"`
	Evented   bool `json:"evented"`
	Full      bool `json:"full"`
}

type CapabilityUpsertRequest struct {
	Descriptor CapabilityDescriptor `json:"descriptor"`
}

type SchemaUpsertRequest struct {
	URI    string                 `json:"uri"`
	Schema map[string]interface{} `json:"schema"`
}

type PolicyValidateRequest struct {
	TenantID   string `json:"tenant_id"`
	Capability string `json:"capability"`
	Action     string `json:"action"`
}

type PolicyValidateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type QuotaRequest struct {
	TenantID       string `json:"tenant_id"`
	MaxInvocations int64  `json:"max_invocations"`
}

type QuotaResponse struct {
	TenantID       string `json:"tenant_id"`
	MaxInvocations int64  `json:"max_invocations"`
}

type AuditRecord struct {
	Actor      string `json:"actor"`
	TenantID   string `json:"tenant_id"`
	Capability string `json:"capability"`
	Outcome    string `json:"outcome"`
	Timestamp  string `json:"timestamp"`
	MessageID  string `json:"message_id"`
}

type UsageSnapshot struct {
	TenantInvocations     map[string]int64 `json:"tenant_invocations"`
	CapabilityInvocations map[string]int64 `json:"capability_invocations"`
	TotalInvocations      int64            `json:"total_invocations"`
}

type ConnectionSnapshot struct {
	ID         string                 `json:"id"`
	Protocol   string                 `json:"protocol"`
	Kind       string                 `json:"kind"`
	TenantID   string                 `json:"tenant_id"`
	Actor      string                 `json:"actor,omitempty"`
	RemoteAddr string                 `json:"remote_addr,omitempty"`
	StartedAt  string                 `json:"started_at"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

type ConnectionSummary struct {
	Total             int            `json:"total"`
	ByProtocol        map[string]int `json:"by_protocol"`
	ByKind            map[string]int `json:"by_kind"`
	ByTenant          map[string]int `json:"by_tenant"`
	NATSBindingActive bool           `json:"nats_binding_active"`
}

type ConnectionsResponse struct {
	GeneratedAt  string               `json:"generated_at"`
	Summary      ConnectionSummary    `json:"summary"`
	Connections  []ConnectionSnapshot `json:"connections"`
	FilterTenant string               `json:"filter_tenant,omitempty"`
	FilterKind   string               `json:"filter_kind,omitempty"`
	FilterProto  string               `json:"filter_protocol,omitempty"`
}

type Org struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tenant struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`
	Name  string `json:"name"`
}

type Gateway struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Region   string `json:"region"`
	Binding  string `json:"binding"`
}
