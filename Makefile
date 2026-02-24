.PHONY: run run-grpc test fmt adapter conformance conformance-grpc conformance-nats proto-toolchain proto-gen demo demo-sse demo-talk-track

run:
	go run ./core/cmd/migd

run-grpc:
	MIGD_GRPC_ADDR=:9090 go run ./core/cmd/migd

adapter:
	go run ./adapters/mcp/cmd/mcp-mig-adapter -mig-url http://localhost:8080 -manifest examples/mcp-mig-adapter.v0.2.manifest.yaml

fmt:
	gofmt -w core/cmd/migd/main.go core/pkg/mig/*.go adapters/mcp/*.go adapters/mcp/cmd/mcp-mig-adapter/main.go conformance/harness/harness_test.go sdk/go/migclient/client.go

test:
	go test ./...

conformance:
	go run ./conformance/harness/cmd/mig-conformance -base-url http://localhost:8080 -tenant acme

conformance-grpc:
	go run ./conformance/harness/cmd/mig-grpc-smoke -addr localhost:9090 -tenant acme

conformance-nats:
	go run ./conformance/harness/cmd/mig-nats-smoke -url nats://localhost:4222 -tenant acme

proto-toolchain:
	./scripts/bootstrap-proto-toolchain.sh

proto-gen:
	./scripts/gen-proto.sh

demo:
	./scripts/demo.sh run

demo-sse:
	./scripts/demo.sh sse

demo-talk-track:
	./scripts/demo.sh talk-track
