package mcp

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Version        string           `yaml:"version"`
	SchemaVersion  string           `yaml:"schema_version,omitempty"`
	DefaultBinding string           `yaml:"default_binding"`
	Mappings       MappingGroup     `yaml:"mappings"`
	Policy         ManifestPolicy   `yaml:"policy"`
	Billing        *ManifestBilling `yaml:"billing,omitempty"`
}

type MappingGroup struct {
	Tools     []ToolMapping     `yaml:"tools"`
	Resources []ResourceMapping `yaml:"resources"`
	Prompts   []PromptMapping   `yaml:"prompts"`
}

type ToolMapping struct {
	MCPName       string   `yaml:"mcp_name"`
	MIGCapability string   `yaml:"mig_capability"`
	Mode          string   `yaml:"mode"`
	TimeoutMS     int      `yaml:"timeout_ms,omitempty"`
	PolicyTags    []string `yaml:"policy_tags,omitempty"`
	BillingTier   string   `yaml:"billing_tier,omitempty"`
}

type ResourceMapping struct {
	MCPURIPattern string `yaml:"mcp_uri_pattern"`
	MIGCapability string `yaml:"mig_capability"`
	Mode          string `yaml:"mode"`
}

type PromptMapping struct {
	MCPName       string `yaml:"mcp_name"`
	MIGCapability string `yaml:"mig_capability"`
	Mode          string `yaml:"mode"`
}

type ManifestPolicy struct {
	AllowStreamingTools    bool   `yaml:"allow_streaming_tools"`
	EmitEventNotifications bool   `yaml:"emit_event_notifications"`
	RequireTraceparent     bool   `yaml:"require_traceparent"`
	TenantResolution       string `yaml:"tenant_resolution"`
}

type ManifestBilling struct {
	Meter      string `yaml:"meter"`
	UsageScope string `yaml:"usage_scope"`
}

func LoadManifest(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Version == "" {
		manifest.Version = "0.1"
	}
	if manifest.DefaultBinding == "" {
		manifest.DefaultBinding = "http"
	}
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = "0.1"
	}
	return manifest, nil
}

func (m Manifest) CapabilityForTool(name string) (ToolMapping, bool) {
	for _, tool := range m.Mappings.Tools {
		if tool.MCPName == name {
			return tool, true
		}
	}
	return ToolMapping{}, false
}
