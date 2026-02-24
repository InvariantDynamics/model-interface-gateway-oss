package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestV2Fields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := []byte(`version: 0.2
schema_version: 0.2
default_binding: http
mappings:
  tools:
    - mcp_name: test_tool
      mig_capability: demo.capability
      mode: unary
      policy_tags: [p1, p2]
      billing_tier: pro
policy:
  allow_streaming_tools: true
  emit_event_notifications: true
  require_traceparent: false
  tenant_resolution: auth_claim
billing:
  meter: invocations_capability_tier
  usage_scope: tenant
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.SchemaVersion != "0.2" {
		t.Fatalf("unexpected schema version: %s", manifest.SchemaVersion)
	}
	tool, ok := manifest.CapabilityForTool("test_tool")
	if !ok {
		t.Fatal("tool mapping not found")
	}
	if tool.BillingTier != "pro" {
		t.Fatalf("unexpected billing tier: %s", tool.BillingTier)
	}
}
