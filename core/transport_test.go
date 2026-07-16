package core

import (
	"context"
	"net/http"
	"testing"
)

func TestTransportResolvePreservesGatewayPathPrefix(t *testing.T) {
	transport, err := NewTransport("https://gateway.example/agent-v2", "key")
	if err != nil {
		t.Fatal(err)
	}
	req, err := transport.NewRequest(context.Background(), "GET", "/api/v1/sandboxes?scope=user", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := req.URL.String(), "https://gateway.example/agent-v2/api/v1/sandboxes?scope=user"; got != want {
		t.Fatalf("request URL = %q, want %q", got, want)
	}
}

func TestTransportNormalizesBaseURLAndAPIKey(t *testing.T) {
	transport, err := NewTransport("  https://gateway.example/agent-v2/  ", "  production-key  ")
	if err != nil {
		t.Fatal(err)
	}
	req, err := transport.NewRequest(context.Background(), http.MethodGet, "/api/v1/sandboxes", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://gateway.example/agent-v2/api/v1/sandboxes" {
		t.Fatalf("request URL = %q", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer production-key" || req.Header.Get("X-API-Key") != "production-key" {
		t.Fatalf("auth headers = %#v", req.Header)
	}
}

func TestTransportResolveKeepsRootGatewayBehavior(t *testing.T) {
	transport, err := NewTransport("https://gateway.example", "key")
	if err != nil {
		t.Fatal(err)
	}
	req, err := transport.NewRequest(context.Background(), "GET", "/api/v1/templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := req.URL.String(), "https://gateway.example/api/v1/templates"; got != want {
		t.Fatalf("request URL = %q, want %q", got, want)
	}
}
