package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	migv01 "github.com/InvariantDynamics/model-interface-gateway-oss/proto/mig/v0_1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {
	addr := flag.String("addr", "localhost:9090", "gRPC address")
	tenant := flag.String("tenant", "acme", "tenant id")
	token := flag.String("token", "", "optional bearer token")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("[FAIL] dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if *token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-tenant-id", *tenant)

	discovery := migv01.NewDiscoveryClient(conn)
	invocation := migv01.NewInvocationClient(conn)

	hello, err := discovery.Hello(ctx, &migv01.HelloRequest{
		Header:            &migv01.MessageHeader{TenantId: *tenant, MigVersion: "0.1"},
		SupportedVersions: []string{"0.1"},
		RequestedBindings: []migv01.BindingType{migv01.BindingType_BINDING_TYPE_GRPC},
	})
	if err != nil {
		fmt.Printf("[FAIL] hello: %v\n", err)
		os.Exit(1)
	}
	if hello.GetSelectedVersion() != "0.1" {
		fmt.Printf("[FAIL] hello selected version mismatch: %s\n", hello.GetSelectedVersion())
		os.Exit(1)
	}
	fmt.Println("[PASS] hello")

	discover, err := discovery.Discover(ctx, &migv01.DiscoverRequest{Header: &migv01.MessageHeader{TenantId: *tenant, MigVersion: "0.1"}})
	if err != nil {
		fmt.Printf("[FAIL] discover: %v\n", err)
		os.Exit(1)
	}
	if len(discover.GetCapabilities()) == 0 {
		fmt.Println("[FAIL] discover: no capabilities")
		os.Exit(1)
	}
	fmt.Println("[PASS] discover")

	payload, _ := structpb.NewStruct(map[string]interface{}{"input": "health-check"})
	invoke, err := invocation.Invoke(ctx, &migv01.InvokeRequest{
		Header:     &migv01.MessageHeader{TenantId: *tenant, MigVersion: "0.1"},
		Capability: "observatory.models.infer",
		Payload:    payload,
	})
	if err != nil {
		fmt.Printf("[FAIL] invoke: %v\n", err)
		os.Exit(1)
	}
	if invoke.GetCapability() == "" {
		fmt.Println("[FAIL] invoke: empty capability")
		os.Exit(1)
	}
	fmt.Println("[PASS] invoke")
}
