#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc not found. Run: make proto-toolchain"
  exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1 || ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "protoc plugins not found in PATH. Add GOPATH/bin to PATH or run: make proto-toolchain"
  exit 1
fi

protoc -I . \
  --go_out=. --go_opt=module=github.com/InvariantDynamics/model-interface-gateway-oss \
  --go-grpc_out=. --go-grpc_opt=module=github.com/InvariantDynamics/model-interface-gateway-oss \
  proto/mig/v0_1/mig.proto

echo "Generated: proto/mig/v0_1/mig.pb.go and proto/mig/v0_1/mig_grpc.pb.go"
