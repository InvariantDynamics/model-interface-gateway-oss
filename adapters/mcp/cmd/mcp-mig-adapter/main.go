package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/InvariantDynamics/model-interface-gateway-oss/adapters/mcp"
)

func main() {
	addr := flag.String("addr", ":8090", "adapter listen address")
	manifestPath := flag.String("manifest", "examples/mcp-mig-adapter.manifest.yaml", "path to adapter manifest")
	migURL := flag.String("mig-url", "http://localhost:8080", "migd base URL")
	migToken := flag.String("mig-token", "", "optional bearer token for MIG backend")
	tenantID := flag.String("tenant-id", "", "optional tenant header for MIG backend")
	flag.Parse()

	manifest, err := mcp.LoadManifest(*manifestPath)
	if err != nil {
		log.Fatalf("failed to load manifest: %v", err)
	}
	adapter := mcp.NewAdapter(*migURL, manifest)
	adapter.SetBearerToken(*migToken)
	adapter.SetTenantID(*tenantID)
	log.Printf("mcp adapter listening on %s", *addr)
	if err := http.ListenAndServe(*addr, adapter); err != nil {
		log.Fatalf("adapter server failed: %v", err)
	}
}
