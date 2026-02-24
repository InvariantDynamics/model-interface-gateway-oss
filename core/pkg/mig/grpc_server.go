package mig

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	migv01 "github.com/InvariantDynamics/model-interface-gateway-oss/proto/mig/v0_1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type grpcServer struct {
	migv01.UnimplementedDiscoveryServer
	migv01.UnimplementedInvocationServer
	migv01.UnimplementedEventsServer
	migv01.UnimplementedControlServer

	svc *Service
}

func RegisterGRPCServices(server *grpc.Server, svc *Service) {
	handler := &grpcServer{svc: svc}
	migv01.RegisterDiscoveryServer(server, handler)
	migv01.RegisterInvocationServer(server, handler)
	migv01.RegisterEventsServer(server, handler)
	migv01.RegisterControlServer(server, handler)
}

func GRPCUnaryAuthInterceptor(cfg AuthConfig) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		principal, err := principalFromGRPCContext(ctx, cfg)
		if err != nil {
			return nil, grpcStatusFromAuthErr(err)
		}
		ctx = context.WithValue(ctx, principalKey{}, principal)
		return handler(ctx, req)
	}
}

func GRPCStreamAuthInterceptor(cfg AuthConfig) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		principal, err := principalFromGRPCContext(ss.Context(), cfg)
		if err != nil {
			return grpcStatusFromAuthErr(err)
		}
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), principalKey{}, principal)}
		return handler(srv, wrapped)
	}
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }

func principalFromGRPCContext(ctx context.Context, cfg AuthConfig) (Principal, error) {
	meta, _ := metadata.FromIncomingContext(ctx)
	authorization := firstMetadataValue(meta, "authorization")
	tenant := firstMetadataValue(meta, "x-tenant-id")
	return PrincipalFromAuthHeaders(authorization, tenant, cfg)
}

func firstMetadataValue(meta metadata.MD, key string) string {
	values := meta.Get(key)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func grpcStatusFromAuthErr(err error) error {
	code := codes.Unauthenticated
	if strings.Contains(strings.ToLower(err.Error()), "tenant") {
		code = codes.PermissionDenied
	}
	return status.Error(code, err.Error())
}

func (g *grpcServer) Hello(ctx context.Context, req *migv01.HelloRequest) (*migv01.HelloResponse, error) {
	principal := principalFromContext(ctx)
	in := HelloRequest{
		Header:            messageHeaderFromProto(req.GetHeader()),
		SupportedVersions: req.GetSupportedVersions(),
		RequestedBindings: bindingTypesFromProto(req.GetRequestedBindings()),
		RequestedFeatures: req.GetRequestedFeatures(),
	}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	out, migErr := g.svc.Hello(in)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	return &migv01.HelloResponse{
		Header:          messageHeaderToProto(out.Header),
		SelectedVersion: out.SelectedVersion,
		SelectedBinding: bindingTypeToProto(out.SelectedBinding),
		EnabledFeatures: out.EnabledFeatures,
		ServerId:        out.ServerID,
	}, nil
}

func (g *grpcServer) Discover(ctx context.Context, req *migv01.DiscoverRequest) (*migv01.DiscoverResponse, error) {
	principal := principalFromContext(ctx)
	in := DiscoverRequest{
		Header:            messageHeaderFromProto(req.GetHeader()),
		Query:             req.GetQuery(),
		IncludeSchemaRefs: req.GetIncludeSchemaRefs(),
		IncludeQoS:        req.GetIncludeQos(),
	}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	out, migErr := g.svc.Discover(in, principal)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	caps := make([]*migv01.CapabilityDescriptor, 0, len(out.Capabilities))
	for _, capDesc := range out.Capabilities {
		caps = append(caps, capabilityToProto(capDesc))
	}
	return &migv01.DiscoverResponse{Header: messageHeaderToProto(out.Header), Capabilities: caps}, nil
}

func (g *grpcServer) Invoke(ctx context.Context, req *migv01.InvokeRequest) (*migv01.InvokeResponse, error) {
	principal := principalFromContext(ctx)
	in := InvokeRequest{
		Header:           messageHeaderFromProto(req.GetHeader()),
		Capability:       req.GetCapability(),
		Payload:          structToMap(req.GetPayload()),
		StreamPreference: streamPreferenceFromProto(req.GetStreamPreference()),
	}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	actor := principal.Subject
	if actor == "" {
		actor = "anonymous"
	}
	out, migErr := g.svc.Invoke(ctx, in.Capability, in, actor, principal)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	return &migv01.InvokeResponse{
		Header:          messageHeaderToProto(out.Header),
		Capability:      out.Capability,
		Payload:         mapToStruct(out.Payload),
		ResultSchemaUri: out.ResultSchemaURI,
	}, nil
}

func (g *grpcServer) StreamInvoke(stream grpc.BidiStreamingServer[migv01.StreamFrame, migv01.StreamFrame]) error {
	principal := principalFromContext(stream.Context())
	if g.svc.metrics != nil {
		g.svc.metrics.IncActiveStream("grpc_bidi")
		defer g.svc.metrics.DecActiveStream("grpc_bidi")
	}
	tenantID := principal.TenantID
	if tenantID == "" {
		tenantID = "unknown"
	}
	_, unregisterConn := g.svc.RegisterConnection(ConnectionSnapshot{
		Protocol:   "grpc",
		Kind:       "stream_invoke",
		TenantID:   tenantID,
		Actor:      principal.Subject,
		RemoteAddr: grpcRemoteAddr(stream.Context()),
		Meta:       map[string]interface{}{"service": "Invocation/StreamInvoke"},
	})
	defer unregisterConn()

	for {
		frame, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		in := streamFrameFromProto(frame)
		if migErr := applyPrincipalHeaderFromPrincipal(&in.Header, principal); migErr != nil {
			if sendErr := stream.Send(streamFrameToProto(StreamFrame{
				Header:     in.Header,
				StreamID:   in.StreamID,
				Capability: in.Capability,
				Kind:       "error",
				EndStream:  true,
				Error:      migErr,
			})); sendErr != nil {
				return sendErr
			}
			continue
		}

		switch in.Kind {
		case "request":
			actor := principal.Subject
			if actor == "" {
				actor = "anonymous"
			}
			invokeReq := InvokeRequest{Header: in.Header, Capability: in.Capability, Payload: in.Payload}
			result, migErr := g.svc.Invoke(stream.Context(), in.Capability, invokeReq, actor, principal)
			response := StreamFrame{
				Header:     in.Header,
				StreamID:   in.StreamID,
				Capability: in.Capability,
				Kind:       "response",
				EndStream:  true,
			}
			if migErr != nil {
				response.Kind = "error"
				response.Error = migErr
			} else {
				response.Payload = result.Payload
			}
			if err := stream.Send(streamFrameToProto(response)); err != nil {
				return err
			}
		case "control":
			cancelReq := CancelRequest{Header: in.Header, TargetMessageID: in.Header.MessageID, Reason: "grpc stream control cancel"}
			ack, migErr := g.svc.Cancel(cancelReq, cancelReq.TargetMessageID)
			response := StreamFrame{Header: in.Header, StreamID: in.StreamID, Capability: in.Capability, Kind: "control", EndStream: true}
			if migErr != nil {
				response.Kind = "error"
				response.Error = migErr
			} else {
				response.Payload = map[string]interface{}{"accepted": ack.Accepted, "status": ack.Status}
			}
			if err := stream.Send(streamFrameToProto(response)); err != nil {
				return err
			}
		default:
			if err := stream.Send(streamFrameToProto(StreamFrame{
				Header:     in.Header,
				StreamID:   in.StreamID,
				Capability: in.Capability,
				Kind:       "error",
				EndStream:  true,
				Error:      &MigError{Code: ErrorInvalidRequest, Message: "frame.kind must be request or control", Retryable: false},
			})); err != nil {
				return err
			}
		}
	}
}

func (g *grpcServer) Publish(ctx context.Context, req *migv01.PublishRequest) (*migv01.PublishAck, error) {
	principal := principalFromContext(ctx)
	in := PublishRequest{
		Header:  messageHeaderFromProto(req.GetHeader()),
		Topic:   req.GetTopic(),
		Key:     req.GetKey(),
		Payload: structToMap(req.GetPayload()),
	}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	out, migErr := g.svc.Publish(in.Topic, in)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	return &migv01.PublishAck{
		Header:   messageHeaderToProto(out.Header),
		Topic:    out.Topic,
		EventId:  out.EventID,
		Sequence: uint64(out.Sequence),
		Accepted: out.Accepted,
	}, nil
}

func (g *grpcServer) Subscribe(req *migv01.SubscribeRequest, stream grpc.ServerStreamingServer[migv01.EventMessage]) error {
	if g.svc.metrics != nil {
		g.svc.metrics.IncActiveStream("grpc_events")
		defer g.svc.metrics.DecActiveStream("grpc_events")
	}
	principal := principalFromContext(stream.Context())
	head := messageHeaderFromProto(req.GetHeader())
	if err := applyPrincipalHeaderFromPrincipal(&head, principal); err != nil {
		return grpcStatusFromMigError(err)
	}
	_, unregisterConn := g.svc.RegisterConnection(ConnectionSnapshot{
		Protocol:   "grpc",
		Kind:       "event_subscribe",
		TenantID:   head.TenantID,
		Actor:      principal.Subject,
		RemoteAddr: grpcRemoteAddr(stream.Context()),
		Meta: map[string]interface{}{
			"service":       "Events/Subscribe",
			"topic":         req.GetTopic(),
			"resume_cursor": req.GetResumeCursor(),
		},
	})
	defer unregisterConn()
	replay, updates, unsubscribe, migErr := g.svc.Subscribe(req.GetTopic(), req.GetResumeCursor())
	if migErr != nil {
		return grpcStatusFromMigError(migErr)
	}
	defer unsubscribe()

	for _, event := range replay {
		if err := stream.Send(eventMessageToProto(event)); err != nil {
			return err
		}
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(eventMessageToProto(event)); err != nil {
				return err
			}
		}
	}
}

func (g *grpcServer) Cancel(ctx context.Context, req *migv01.CancelRequest) (*migv01.CancelAck, error) {
	principal := principalFromContext(ctx)
	in := CancelRequest{
		Header:          messageHeaderFromProto(req.GetHeader()),
		TargetMessageID: req.GetTargetMessageId(),
		Reason:          req.GetReason(),
	}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	out, migErr := g.svc.Cancel(in, in.TargetMessageID)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	return &migv01.CancelAck{
		Header:          messageHeaderToProto(out.Header),
		TargetMessageId: out.TargetMessageID,
		Accepted:        out.Accepted,
		Status:          out.Status,
	}, nil
}

func (g *grpcServer) Heartbeat(ctx context.Context, req *migv01.HeartbeatRequest) (*migv01.HeartbeatAck, error) {
	principal := principalFromContext(ctx)
	in := HeartbeatRequest{Header: messageHeaderFromProto(req.GetHeader()), IntervalMS: int(req.GetIntervalMs())}
	if err := applyPrincipalHeaderFromPrincipal(&in.Header, principal); err != nil {
		return nil, grpcStatusFromMigError(err)
	}
	out, migErr := g.svc.Heartbeat(in)
	if migErr != nil {
		return nil, grpcStatusFromMigError(migErr)
	}
	return &migv01.HeartbeatAck{
		Header:              messageHeaderToProto(out.Header),
		SuggestedIntervalMs: uint32(out.SuggestedIntervalMS),
		LoadFactor:          out.LoadFactor,
	}, nil
}

func applyPrincipalHeaderFromPrincipal(head *MessageHeader, principal Principal) *MigError {
	if head.Meta == nil {
		head.Meta = map[string]interface{}{}
	}
	head.Meta["idg.principal_subject"] = principal.Subject
	head.Meta["idg.principal_scopes"] = principal.ScopesList()
	if head.TenantID == "" && principal.TenantID != "" {
		head.TenantID = principal.TenantID
	}
	if principal.TenantID != "" && head.TenantID != principal.TenantID {
		return &MigError{Code: ErrorForbidden, Message: "tenant_id does not match authenticated principal", Retryable: false}
	}
	if head.TenantID == "" {
		return &MigError{Code: ErrorInvalidRequest, Message: "tenant_id is required", Retryable: false}
	}
	return nil
}

func grpcStatusFromMigError(err *MigError) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch err.Code {
	case ErrorInvalidRequest:
		code = codes.InvalidArgument
	case ErrorUnauthorized:
		code = codes.Unauthenticated
	case ErrorForbidden:
		code = codes.PermissionDenied
	case ErrorNotFound, ErrorUnsupportedCapability:
		code = codes.NotFound
	case ErrorVersionMismatch:
		code = codes.FailedPrecondition
	case ErrorTimeout:
		code = codes.DeadlineExceeded
	case ErrorRateLimited, ErrorBackpressure:
		code = codes.ResourceExhausted
	case ErrorUnavailable:
		code = codes.Unavailable
	case ErrorInternal:
		code = codes.Internal
	}
	return status.Error(code, fmt.Sprintf("%s: %s", err.Code, err.Message))
}

func messageHeaderFromProto(in *migv01.MessageHeader) MessageHeader {
	if in == nil {
		return MessageHeader{}
	}
	meta := map[string]interface{}{}
	if in.Meta != nil {
		meta = in.Meta.AsMap()
	}
	timestamp := ""
	if in.Timestamp != nil {
		timestamp = in.Timestamp.AsTime().UTC().Format(time.RFC3339)
	}
	return MessageHeader{
		MIGVersion:     in.GetMigVersion(),
		MessageID:      in.GetMessageId(),
		Timestamp:      timestamp,
		TenantID:       in.GetTenantId(),
		SessionID:      in.GetSessionId(),
		Traceparent:    in.GetTraceparent(),
		IdempotencyKey: in.GetIdempotencyKey(),
		DeadlineMS:     int(in.GetDeadlineMs()),
		Meta:           meta,
	}
}

func messageHeaderToProto(in MessageHeader) *migv01.MessageHeader {
	timestamp, err := time.Parse(time.RFC3339, in.Timestamp)
	if err != nil {
		timestamp = time.Now().UTC()
	}
	meta := mapToStruct(in.Meta)
	return &migv01.MessageHeader{
		MigVersion:     in.MIGVersion,
		MessageId:      in.MessageID,
		Timestamp:      timestamppb.New(timestamp),
		TenantId:       in.TenantID,
		SessionId:      in.SessionID,
		Traceparent:    in.Traceparent,
		IdempotencyKey: in.IdempotencyKey,
		DeadlineMs:     uint32(in.DeadlineMS),
		Meta:           meta,
	}
}

func bindingTypesFromProto(in []migv01.BindingType) []string {
	out := make([]string, 0, len(in))
	for _, b := range in {
		out = append(out, bindingTypeFromProto(b))
	}
	return out
}

func bindingTypeFromProto(binding migv01.BindingType) string {
	switch binding {
	case migv01.BindingType_BINDING_TYPE_GRPC:
		return "grpc"
	case migv01.BindingType_BINDING_TYPE_NATS:
		return "nats"
	case migv01.BindingType_BINDING_TYPE_HTTP:
		return "http"
	default:
		return ""
	}
}

func bindingTypeToProto(binding string) migv01.BindingType {
	switch strings.ToLower(strings.TrimSpace(binding)) {
	case "grpc":
		return migv01.BindingType_BINDING_TYPE_GRPC
	case "nats":
		return migv01.BindingType_BINDING_TYPE_NATS
	case "http":
		return migv01.BindingType_BINDING_TYPE_HTTP
	default:
		return migv01.BindingType_BINDING_TYPE_UNSPECIFIED
	}
}

func capabilityToProto(capability CapabilityDescriptor) *migv01.CapabilityDescriptor {
	modes := make([]migv01.InvocationMode, 0, len(capability.Modes))
	for _, mode := range capability.Modes {
		modes = append(modes, invocationModeToProto(mode))
	}
	return &migv01.CapabilityDescriptor{
		Id:              capability.ID,
		Version:         capability.Version,
		Modes:           modes,
		InputSchemaUri:  capability.InputSchemaURI,
		OutputSchemaUri: capability.OutputSchemaURI,
		EventTopics:     capability.EventTopics,
		AuthScopes:      capability.AuthScopes,
		Qos: &migv01.QoSProfile{
			MaxPayloadBytes:   uint64(capability.QoS.MaxPayloadBytes),
			SupportsReplay:    capability.QoS.SupportsReplay,
			DeliverySemantics: deliverySemanticsToProto(capability.QoS.DeliverySemantics),
			SupportsOrdering:  capability.QoS.SupportsOrdering,
		},
	}
}

func streamPreferenceFromProto(pref migv01.StreamPreference) string {
	switch pref {
	case migv01.StreamPreference_STREAM_PREFERENCE_UNARY:
		return "unary"
	case migv01.StreamPreference_STREAM_PREFERENCE_SERVER_STREAM:
		return "server_stream"
	case migv01.StreamPreference_STREAM_PREFERENCE_BIDI_STREAM:
		return "bidi_stream"
	default:
		return ""
	}
}

func invocationModeToProto(mode string) migv01.InvocationMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "unary":
		return migv01.InvocationMode_INVOCATION_MODE_UNARY
	case "server_stream":
		return migv01.InvocationMode_INVOCATION_MODE_SERVER_STREAM
	case "client_stream":
		return migv01.InvocationMode_INVOCATION_MODE_CLIENT_STREAM
	case "bidi_stream":
		return migv01.InvocationMode_INVOCATION_MODE_BIDI_STREAM
	default:
		return migv01.InvocationMode_INVOCATION_MODE_UNSPECIFIED
	}
}

func deliverySemanticsToProto(value string) migv01.DeliverySemantics {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "best_effort":
		return migv01.DeliverySemantics_DELIVERY_SEMANTICS_BEST_EFFORT
	case "at_least_once":
		return migv01.DeliverySemantics_DELIVERY_SEMANTICS_AT_LEAST_ONCE
	case "exactly_once":
		return migv01.DeliverySemantics_DELIVERY_SEMANTICS_EXACTLY_ONCE
	default:
		return migv01.DeliverySemantics_DELIVERY_SEMANTICS_UNSPECIFIED
	}
}

func streamFrameFromProto(in *migv01.StreamFrame) StreamFrame {
	if in == nil {
		return StreamFrame{}
	}
	return StreamFrame{
		Header:     messageHeaderFromProto(in.GetHeader()),
		StreamID:   in.GetStreamId(),
		Capability: in.GetCapability(),
		Kind:       frameKindFromProto(in.GetKind()),
		Payload:    structToMap(in.GetPayload()),
		EndStream:  in.GetEndStream(),
		Error:      migErrorFromProto(in.GetError()),
	}
}

func streamFrameToProto(in StreamFrame) *migv01.StreamFrame {
	return &migv01.StreamFrame{
		Header:     messageHeaderToProto(in.Header),
		StreamId:   in.StreamID,
		Capability: in.Capability,
		Kind:       frameKindToProto(in.Kind),
		Payload:    mapToStruct(in.Payload),
		EndStream:  in.EndStream,
		Error:      migErrorToProto(in.Error),
	}
}

func frameKindFromProto(kind migv01.FrameKind) string {
	switch kind {
	case migv01.FrameKind_FRAME_KIND_REQUEST:
		return "request"
	case migv01.FrameKind_FRAME_KIND_RESPONSE:
		return "response"
	case migv01.FrameKind_FRAME_KIND_EVENT:
		return "event"
	case migv01.FrameKind_FRAME_KIND_CONTROL:
		return "control"
	case migv01.FrameKind_FRAME_KIND_ERROR:
		return "error"
	default:
		return ""
	}
}

func frameKindToProto(kind string) migv01.FrameKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "request":
		return migv01.FrameKind_FRAME_KIND_REQUEST
	case "response":
		return migv01.FrameKind_FRAME_KIND_RESPONSE
	case "event":
		return migv01.FrameKind_FRAME_KIND_EVENT
	case "control":
		return migv01.FrameKind_FRAME_KIND_CONTROL
	case "error":
		return migv01.FrameKind_FRAME_KIND_ERROR
	default:
		return migv01.FrameKind_FRAME_KIND_UNSPECIFIED
	}
}

func eventMessageToProto(event EventMessage) *migv01.EventMessage {
	publishedAt, err := time.Parse(time.RFC3339, event.PublishedAt)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	return &migv01.EventMessage{
		Header:      messageHeaderToProto(event.Header),
		Topic:       event.Topic,
		EventId:     event.EventID,
		Sequence:    uint64(event.Sequence),
		Payload:     mapToStruct(event.Payload),
		PublishedAt: timestamppb.New(publishedAt),
		Replay:      event.Replay,
	}
}

func migErrorFromProto(err *migv01.MigError) *MigError {
	if err == nil {
		return nil
	}
	return &MigError{
		Code:      migErrorCodeFromProto(err.GetCode()),
		Message:   err.GetMessage(),
		Retryable: err.GetRetryable(),
		Details:   structToMap(err.GetDetails()),
	}
}

func migErrorToProto(err *MigError) *migv01.MigError {
	if err == nil {
		return nil
	}
	return &migv01.MigError{
		Code:      migErrorCodeToProto(err.Code),
		Message:   err.Message,
		Retryable: err.Retryable,
		Details:   mapToStruct(err.Details),
	}
}

func migErrorCodeFromProto(code migv01.MigErrorCode) string {
	switch code {
	case migv01.MigErrorCode_MIG_INVALID_REQUEST:
		return ErrorInvalidRequest
	case migv01.MigErrorCode_MIG_UNAUTHORIZED:
		return ErrorUnauthorized
	case migv01.MigErrorCode_MIG_FORBIDDEN:
		return ErrorForbidden
	case migv01.MigErrorCode_MIG_NOT_FOUND:
		return ErrorNotFound
	case migv01.MigErrorCode_MIG_UNSUPPORTED_CAPABILITY:
		return ErrorUnsupportedCapability
	case migv01.MigErrorCode_MIG_VERSION_MISMATCH:
		return ErrorVersionMismatch
	case migv01.MigErrorCode_MIG_TIMEOUT:
		return ErrorTimeout
	case migv01.MigErrorCode_MIG_RATE_LIMITED:
		return ErrorRateLimited
	case migv01.MigErrorCode_MIG_BACKPRESSURE:
		return ErrorBackpressure
	case migv01.MigErrorCode_MIG_UNAVAILABLE:
		return ErrorUnavailable
	case migv01.MigErrorCode_MIG_INTERNAL:
		return ErrorInternal
	default:
		return ErrorInternal
	}
}

func migErrorCodeToProto(code string) migv01.MigErrorCode {
	switch code {
	case ErrorInvalidRequest:
		return migv01.MigErrorCode_MIG_INVALID_REQUEST
	case ErrorUnauthorized:
		return migv01.MigErrorCode_MIG_UNAUTHORIZED
	case ErrorForbidden:
		return migv01.MigErrorCode_MIG_FORBIDDEN
	case ErrorNotFound:
		return migv01.MigErrorCode_MIG_NOT_FOUND
	case ErrorUnsupportedCapability:
		return migv01.MigErrorCode_MIG_UNSUPPORTED_CAPABILITY
	case ErrorVersionMismatch:
		return migv01.MigErrorCode_MIG_VERSION_MISMATCH
	case ErrorTimeout:
		return migv01.MigErrorCode_MIG_TIMEOUT
	case ErrorRateLimited:
		return migv01.MigErrorCode_MIG_RATE_LIMITED
	case ErrorBackpressure:
		return migv01.MigErrorCode_MIG_BACKPRESSURE
	case ErrorUnavailable:
		return migv01.MigErrorCode_MIG_UNAVAILABLE
	case ErrorInternal:
		return migv01.MigErrorCode_MIG_INTERNAL
	default:
		return migv01.MigErrorCode_MIG_INTERNAL
	}
}

func structToMap(in *structpb.Struct) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	return in.AsMap()
}

func mapToStruct(in map[string]interface{}) *structpb.Struct {
	if len(in) == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	st, err := structpb.NewStruct(in)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return st
}

func grpcRemoteAddr(ctx context.Context) string {
	info, ok := peer.FromContext(ctx)
	if !ok || info.Addr == nil {
		return ""
	}
	return info.Addr.String()
}

func StartGRPCServer(ctx context.Context, addr string, svc *Service, authCfg AuthConfig) (*grpc.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen grpc: %w", err)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(GRPCUnaryAuthInterceptor(authCfg)),
		grpc.StreamInterceptor(GRPCStreamAuthInterceptor(authCfg)),
	)
	RegisterGRPCServices(server, svc)
	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()
	go func() {
		_ = server.Serve(listener)
	}()
	return server, listener, nil
}
