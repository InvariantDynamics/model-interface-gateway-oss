package mig

import "testing"

func TestStartNATSBindingRequiresConnection(t *testing.T) {
	svc, err := NewServiceWithOptions(ServiceOptions{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()
	if _, err := svc.StartNATSBinding(); err == nil {
		t.Fatal("expected error when nats connection is not configured")
	}
}
