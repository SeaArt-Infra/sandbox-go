package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sandbox "github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/build"
)

func TestCreateWaitReadyGetsIDBeforePolling(t *testing.T) {
	var detailCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sandboxes":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if waitReady, ok := body["waitReady"].(bool); !ok || waitReady {
				t.Fatalf("wire waitReady = %#v, want false", body["waitReady"])
			}
			if autoPause, ok := body["autoPause"].(bool); !ok || !autoPause {
				t.Fatalf("wire autoPause = %#v, want true", body["autoPause"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sandboxID":"sb-1","status":"starting","state":"creating"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sandboxes/sb-1":
			call := detailCalls.Add(1)
			if call == 1 {
				_, _ = w.Write([]byte(`{"sandboxID":"sb-1","status":"starting","state":"creating"}`))
				return
			}
			_, _ = w.Write([]byte(`{"sandboxID":"sb-1","status":"running","state":"active","envdUrl":"https://runtime.example","envdAccessToken":"runtime-token"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	wait := true
	autoPause := true
	created, err := client.Create(context.Background(), "tpl-1", &sandbox.CreateOptions{
		WaitReady: &wait, AutoPause: &autoPause, WaitTimeout: time.Second, PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SandboxID != "sb-1" || created.State != "active" || created.EnvdURL == nil || *created.EnvdURL != "https://runtime.example" {
		t.Fatalf("created = %#v", created.Sandbox)
	}
}

func TestCreateWaitReadyCancellationReturnsResourceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sandboxID":"sb-cancel","status":"starting","state":"creating"}`))
			return
		}
		_, _ = w.Write([]byte(`{"sandboxID":"sb-cancel","status":"starting","state":"creating"}`))
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	wait := true
	created, err := client.Create(context.Background(), "tpl-1", &sandbox.CreateOptions{
		WaitReady: &wait, WaitTimeout: 15 * time.Millisecond, PollInterval: 5 * time.Millisecond,
	})
	if created == nil || created.SandboxID != "sb-cancel" {
		t.Fatalf("partial sandbox = %#v", created)
	}
	var operationErr *sandbox.ResourceOperationError
	if !errors.As(err, &operationErr) || operationErr.ResourceID != "sb-cancel" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %#v", err)
	}
}

func TestBuildTemplateUsesFlatExtensionsAndCleansUpBeforeAcceptance(t *testing.T) {
	var deleteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/templates":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			extensions, ok := body["extensions"].(map[string]any)
			if !ok || extensions["baseTemplateID"] != "tpl-base" || extensions["visibility"] != "team" || extensions["workdir"] != "/workspace" {
				t.Fatalf("extensions = %#v", body["extensions"])
			}
			if _, stale := extensions["seacloud"]; stale {
				t.Fatalf("stale nested extensions = %#v", extensions)
			}
			if envs, ok := extensions["envs"].(map[string]any); !ok || envs["RUNTIME"] != "1" {
				t.Fatalf("envs = %#v", extensions["envs"])
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-partial","names":["demo"]}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/builds/"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"invalid build request"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/templates/tpl-partial":
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	info, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("alpine:3.20"), "demo", &sandbox.TemplateBuildOptions{
		BaseTemplateID: "tpl-base",
		Visibility:     "team",
		Envs:           map[string]string{"RUNTIME": "1"},
		Workdir:        "/workspace",
		VolumeMounts:   []build.TemplateVolumeMount{{Name: "workspace", Path: "/workspace", StorageType: "nfs"}},
	})
	if info == nil || info.TemplateID != "tpl-partial" || info.BuildID == "" {
		t.Fatalf("partial build info = %#v", info)
	}
	var operationErr *sandbox.ResourceOperationError
	if !errors.As(err, &operationErr) || !operationErr.CleanupAttempted || operationErr.CleanupErr != nil || deleteCalls.Load() != 1 {
		t.Fatalf("error/deleteCalls = %#v/%d", err, deleteCalls.Load())
	}
}

func TestBuildTemplateAmbiguousAcceptancePreservesTemplate(t *testing.T) {
	var deleteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/templates":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-ambiguous"}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/builds/"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"acceptance state unknown"}`))
		case r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	info, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("alpine:3.20"), "demo", nil)
	if info == nil || info.TemplateID != "tpl-ambiguous" || info.BuildID == "" {
		t.Fatalf("partial build info = %#v", info)
	}
	var operationErr *sandbox.ResourceOperationError
	if !errors.As(err, &operationErr) || operationErr.CleanupAttempted || deleteCalls.Load() != 0 {
		t.Fatalf("error/deleteCalls = %#v/%d", err, deleteCalls.Load())
	}
}

func TestBuildTemplateWaitTimeoutPreservesAcceptedBuildWithoutCleanup(t *testing.T) {
	var deleteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/templates":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-building"}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/builds/"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"templateID":"tpl-building","status":"building","logEntries":[]}`))
		case r.Method == http.MethodDelete:
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	info, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("alpine:3.20"), "demo", &sandbox.TemplateBuildOptions{
		WaitTimeout:  15 * time.Millisecond,
		PollInterval: 5 * time.Millisecond,
	})
	if info == nil || info.TemplateID != "tpl-building" || info.BuildID == "" {
		t.Fatalf("partial build info = %#v", info)
	}
	if !errors.Is(err, context.DeadlineExceeded) || deleteCalls.Load() != 0 {
		t.Fatalf("error/deleteCalls = %v/%d", err, deleteCalls.Load())
	}
}

func TestBuildTemplateTerminalFailureReturnsBuildAndOperationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/templates":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-failed","names":["demo"]}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/builds/"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"templateID":"tpl-failed","buildID":"build-failed","status":"error","reason":{"message":"npm ci failed"},"logEntries":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/templates/tpl-failed":
			_, _ = w.Write([]byte(`{"templateID":"tpl-failed","public":false,"aliases":[],"names":["demo"],"builds":[]}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/builds/"):
			_, _ = w.Write([]byte(`{"templateID":"tpl-failed","buildID":"build-failed","status":"error","errorMessage":"npm ci failed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	info, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("node:20"), "demo", nil)
	if info == nil || info.TemplateID != "tpl-failed" || info.BuildID == "" || info.Status != "error" || info.Build == nil {
		t.Fatalf("failed build info = %#v", info)
	}
	var operationErr *sandbox.ResourceOperationError
	if !errors.As(err, &operationErr) {
		t.Fatalf("error = %T %v, want ResourceOperationError", err, err)
	}
	if operationErr.ResourceID != "tpl-failed" || operationErr.RelatedID != info.BuildID || !strings.Contains(err.Error(), "npm ci failed") {
		t.Fatalf("operation error = %#v (%v)", operationErr, err)
	}
}

func TestBuildTemplatePreservesTerminalStatusWhenDetailFetchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/templates":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-terminal"}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/builds/"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"templateID":"tpl-terminal","status":"failed","reason":{"message":"compile failed"},"logEntries":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/templates/tpl-terminal":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"temporary read failure"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	info, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("node:20"), "demo", nil)
	if info == nil || info.TemplateID != "tpl-terminal" || info.Status != "failed" {
		t.Fatalf("terminal partial info = %#v", info)
	}
	var operationErr *sandbox.ResourceOperationError
	if !errors.As(err, &operationErr) || operationErr.Operation != "get completed template" {
		t.Fatalf("error = %#v", err)
	}
}
