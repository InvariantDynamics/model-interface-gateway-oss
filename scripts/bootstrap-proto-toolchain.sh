#!/usr/bin/env bash
set -euo pipefail

if ! command -v protoc >/dev/null 2>&1; then
  cat <<MSG
protoc is not installed.
Install with one of:
  - brew install protobuf
  - apt-get install -y protobuf-compiler
  - download from https://github.com/protocolbuffers/protobuf/releases
MSG
  exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo "protoc: $(protoc --version)"
echo "protoc-gen-go: $(command -v protoc-gen-go)"
echo "protoc-gen-go-grpc: $(command -v protoc-gen-go-grpc)"
