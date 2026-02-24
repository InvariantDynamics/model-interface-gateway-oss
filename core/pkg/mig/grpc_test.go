package mig

import (
	"context"
	"net"
	"testing"
	"time"

	migv01 "github.com/mig-standard/mig/proto/mig/v0_1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGRPCHelloDiscoverInvoke(t *testing.T) {
	svc := NewService()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(GRPCUnaryAuthInterceptor(AuthConfig{Mode: AuthModeNone})),
		grpc.StreamInterceptor(GRPCStreamAuthInterceptor(AuthConfig{Mode: AuthModeNone})),
	)
	RegisterGRPCServices(server, svc)
	defer server.Stop()

	go func() {
		_ = server.Serve(listener)
	}()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	defer conn.Close()

	discovery := migv01.NewDiscoveryClient(conn)
	invocation := migv01.NewInvocationClient(conn)

	helloResp, err := discovery.Hello(ctx, &migv01.HelloRequest{
		Header:            &migv01.MessageHeader{TenantId: "acme", MigVersion: "0.1"},
		SupportedVersions: []string{"0.1"},
		RequestedBindings: []migv01.BindingType{migv01.BindingType_BINDING_TYPE_GRPC},
	})
	if err != nil {
		t.Fatalf("hello failed: %v", err)
	}
	if helloResp.GetSelectedVersion() != "0.1" {
		t.Fatalf("unexpected selected version: %s", helloResp.GetSelectedVersion())
	}

	discoverResp, err := discovery.Discover(ctx, &migv01.DiscoverRequest{
		Header: &migv01.MessageHeader{TenantId: "acme", MigVersion: "0.1"},
	})
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(discoverResp.GetCapabilities()) == 0 {
		t.Fatal("expected at least one capability")
	}

	payload, _ := structpb.NewStruct(map[string]interface{}{"input": "hello"})
	invokeResp, err := invocation.Invoke(ctx, &migv01.InvokeRequest{
		Header:     &migv01.MessageHeader{TenantId: "acme", MigVersion: "0.1"},
		Capability: "observatory.models.infer",
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if invokeResp.GetCapability() != "observatory.models.infer" {
		t.Fatalf("unexpected capability: %s", invokeResp.GetCapability())
	}
}

func TestGRPCStreamInvoke(t *testing.T) {
	svc := NewService()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(GRPCUnaryAuthInterceptor(AuthConfig{Mode: AuthModeNone})),
		grpc.StreamInterceptor(GRPCStreamAuthInterceptor(AuthConfig{Mode: AuthModeNone})),
	)
	RegisterGRPCServices(server, svc)
	defer server.Stop()
	go func() {
		_ = server.Serve(listener)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	defer conn.Close()

	client := migv01.NewInvocationClient(conn)
	stream, err := client.StreamInvoke(ctx)
	if err != nil {
		t.Fatalf("stream invoke: %v", err)
	}
	payload, _ := structpb.NewStruct(map[string]interface{}{"input": "hello"})
	if err := stream.Send(&migv01.StreamFrame{
		Header:     &migv01.MessageHeader{TenantId: "acme", MigVersion: "0.1", MessageId: "m1"},
		StreamId:   "s1",
		Capability: "observatory.models.infer",
		Kind:       migv01.FrameKind_FRAME_KIND_REQUEST,
		Payload:    payload,
	}); err != nil {
		t.Fatalf("stream send: %v", err)
	}
	frame, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream recv: %v", err)
	}
	if frame.GetKind() != migv01.FrameKind_FRAME_KIND_RESPONSE {
		t.Fatalf("expected response frame, got %s", frame.GetKind().String())
	}
	if !frame.GetEndStream() {
		t.Fatal("expected end_stream true")
	}
}
