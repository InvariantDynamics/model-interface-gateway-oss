package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	url := flag.String("url", nats.DefaultURL, "NATS url")
	tenant := flag.String("tenant", "acme", "tenant id")
	flag.Parse()

	nc, err := nats.Connect(*url)
	if err != nil {
		fmt.Printf("[FAIL] connect: %v\n", err)
		os.Exit(1)
	}
	defer nc.Close()

	checks := []struct {
		name    string
		subject string
		body    map[string]interface{}
	}{
		{
			name:    "hello",
			subject: fmt.Sprintf("mig.v0_1.%s.hello", *tenant),
			body: map[string]interface{}{
				"header":             map[string]interface{}{"tenant_id": *tenant},
				"supported_versions": []string{"0.1"},
				"requested_bindings": []string{"nats"},
			},
		},
		{
			name:    "discover",
			subject: fmt.Sprintf("mig.v0_1.%s.discover", *tenant),
			body: map[string]interface{}{
				"header": map[string]interface{}{"tenant_id": *tenant},
			},
		},
		{
			name:    "invoke",
			subject: fmt.Sprintf("mig.v0_1.%s.invoke.observatory.models.infer", *tenant),
			body: map[string]interface{}{
				"header":  map[string]interface{}{"tenant_id": *tenant},
				"payload": map[string]interface{}{"input": "health-check"},
			},
		},
	}

	for _, check := range checks {
		payload, _ := json.Marshal(check.body)
		msg, err := nc.Request(check.subject, payload, 5*time.Second)
		if err != nil {
			fmt.Printf("[FAIL] %s: %v\n", check.name, err)
			os.Exit(1)
		}
		if len(msg.Data) == 0 {
			fmt.Printf("[FAIL] %s: empty response\n", check.name)
			os.Exit(1)
		}
		var asMap map[string]interface{}
		if err := json.Unmarshal(msg.Data, &asMap); err != nil {
			fmt.Printf("[FAIL] %s: invalid json response: %v\n", check.name, err)
			os.Exit(1)
		}
		if _, hasErr := asMap["error"]; hasErr {
			fmt.Printf("[FAIL] %s: %s\n", check.name, string(msg.Data))
			os.Exit(1)
		}
		fmt.Printf("[PASS] %s\n", check.name)
	}
}
