package tests

import (
	"testing"

	"github.com/SeaArt-Infra/sandbox-go"
)

func TestRootClientInitializesBuildAndRuntime(t *testing.T) {
	client, err := sandbox.NewClient("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Build == nil {
		t.Fatal("build handle is nil")
	}

	runtime, err := client.Runtime("https://sandbox-gateway.cloud.seaart.ai", "unit-runtime-auth")
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}
	if got := runtime.BaseURL(); got != "https://sandbox-gateway.cloud.seaart.ai" {
		t.Fatalf("runtime baseURL = %q", got)
	}
}
