package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func TestClientHighLevelHelpersReuseStoredConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-1" {
				t.Fatalf("project header = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode sandbox request: %v", err)
			}
			if req["templateID"] != "base" {
				t.Fatalf("templateID = %#v", req["templateID"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-high",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2026-01-01T00:00:00Z",
				"endAt":"2026-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/sandboxes/sb-high" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-high",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2026-01-01T00:00:00Z",
				"endAt":"2026-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-1" {
				t.Fatalf("project header = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode template request: %v", err)
			}
			if req["name"] != "demo" {
				t.Fatalf("name = %#v", req["name"])
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v1"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-1","status":"ready","logs":[],"logEntries":[]}`))
		case r.URL.Path == "/api/v1/templates/tpl-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-1","status":"ready","image":"demo:v1","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := sandbox.NewClient(server.URL, "unit-auth-value", core.WithProjectID("project-1"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	waitReady := true
	created, err := client.Create(context.Background(), "base", &sandbox.CreateOptions{WaitReady: &waitReady})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	detail, err := created.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if created.SandboxID != "sb-high" || detail.SandboxID != "sb-high" {
		t.Fatalf("sandbox = %#v detail = %#v", created, detail)
	}

	template := sandbox.NewTemplate().FromBaseImage().RunCmd("echo hello", nil)
	built, err := client.BuildTemplate(context.Background(), template, "demo:v1", &sandbox.TemplateBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if built.TemplateID != "tpl-1" || built.Status != "ready" {
		t.Fatalf("built = %#v", built)
	}
}
