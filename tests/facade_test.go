package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
)

func newClient(t *testing.T, baseURL string) *sandbox.Client {
	t.Helper()

	client, err := sandbox.NewClient(baseURL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestFacadeCreateSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes":
			w.WriteHeader(http.StatusCreated)
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["templateID"] != "base" {
				t.Fatalf("templateID = %#v", req["templateID"])
			}
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/run":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode runtime request: %v", err)
			}
			if req["cmd"] != "echo hello" {
				t.Fatalf("cmd = %#v", req["cmd"])
			}
			_, _ = w.Write([]byte(`{"stdout":"hello\n","stderr":"","exit_code":0,"duration_ms":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	commands, err := created.Commands()
	if err != nil {
		t.Fatalf("Commands: %v", err)
	}
	result, err := commands.Run(context.Background(), "echo hello", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("result = %#v", result)
	}
	execResult, err := commands.Exec(context.Background(), "echo hello", nil)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if execResult.ExitCode != 0 || strings.TrimSpace(execResult.Stdout) != "hello" {
		t.Fatalf("execResult = %#v", execResult)
	}
}

func TestFacadeGitModule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-git",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/run":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode runtime request: %v", err)
			}
			if req["cmd"] != "git" {
				t.Fatalf("cmd = %#v", req["cmd"])
			}
			args, _ := req["args"].([]any)
			got := make([]string, 0, len(args))
			for _, item := range args {
				got = append(got, item.(string))
			}
			want := []string{"clone", "--branch", "main", "--depth", "1", "https://github.com/acme/repo.git", "/workspace/repo"}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("args = %#v", got)
			}
			_, _ = w.Write([]byte(`{"stdout":"ok\n","stderr":"","exit_code":0,"duration_ms":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	git, err := created.Git()
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	result, err := git.Clone(context.Background(), "https://github.com/acme/repo.git", "/workspace/repo", &sandbox.GitCloneOptions{
		Branch: "main",
		Depth:  1,
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "ok" {
		t.Fatalf("result = %#v", result)
	}
}

func TestFacadeSandboxLifecycleHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-helpers",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"status":"paused",
				"state":"paused",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/sandboxes/sb-helpers" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-helpers",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"cpuCount":2,
				"memoryMB":1024,
				"diskSizeMB":2048,
				"status":"paused",
				"state":"paused",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/sandboxes/sb-helpers/connect" && r.Method == http.MethodPost:
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode connect request: %v", err)
			}
			if req["timeout"] != float64(300) {
				t.Fatalf("timeout = %#v", req["timeout"])
			}
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-helpers",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/metrics" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"cpu_used_pct":1.5,"mem_used_mib":64,"mem_total_mib":1024,"disk_used":128,"disk_total":4096}`))
		case r.URL.Path == "/api/v1/sandboxes/sb-helpers" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.IsRunning() {
		t.Fatal("created sandbox should be paused")
	}

	detail, err := created.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if detail.Status != "paused" || detail.IsRunning() {
		t.Fatalf("detail = %#v", detail)
	}

	resumed, err := created.Resume(context.Background(), 0)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if !resumed.IsRunning() {
		t.Fatalf("resumed = %#v", resumed)
	}

	metrics, err := resumed.GetMetrics(context.Background())
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if metrics.CPUUsedPct != 1.5 || metrics.MemUsedMiB != 64 {
		t.Fatalf("metrics = %#v", metrics)
	}

	if err := resumed.Kill(context.Background()); err != nil {
		t.Fatalf("Kill: %v", err)
	}
}

func TestTemplateFacadeBuildsAndPolls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["name"] != "demo" {
				t.Fatalf("name = %#v", req["name"])
			}
			extensions, ok := req["extensions"].(map[string]any)
			if !ok {
				t.Fatalf("extensions = %#v", req["extensions"])
			}
			if extensions["baseTemplateID"] != "tpl-base-1" || extensions["visibility"] != "personal" {
				t.Fatalf("extensions = %#v", extensions)
			}
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v1"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{
				"buildID":"build-x",
				"templateID":"tpl-1",
				"status":"ready",
				"logs":[],
				"logEntries":[{"timestamp":"2026-01-01T00:00:00Z","level":"info","step":"RUN","message":"installed dependencies"}],
				"createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-01T00:00:01Z"
			}`))
		case r.URL.Path == "/api/v1/templates/tpl-1":
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"build-x","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-x","templateID":"tpl-1","status":"ready","image":"demo:v1","errorMessage":"","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	logs := make([]string, 0, 4)
	template := sandbox.NewTemplate().
		FromImage("docker.io/library/node:20").
		RunCmd("npm install", nil).
		SetStartCmd("npm start", sandbox.WaitForPort(3000))

	client := newClient(t, server.URL)
	built, err := client.BuildTemplate(context.Background(), template, "demo:v1", &sandbox.TemplateBuildOptions{
		BaseTemplateID: "tpl-base-1",
		PollInterval:   0,
		OnBuildLog: func(entry sandbox.LogEntry) {
			logs = append(logs, entry.String())
		},
	})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if built.TemplateID != "tpl-1" || built.Status != "ready" {
		t.Fatalf("built = %#v", built)
	}
	if len(logs) == 0 || !strings.Contains(strings.Join(logs, "\n"), "installed dependencies") {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestTemplateBuildInBackgroundSkipsPolling(t *testing.T) {
	statusRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-bg","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v2"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			statusRequested = true
			t.Fatalf("status should not be requested for background builds")
		case r.URL.Path == "/api/v1/templates/tpl-bg":
			_, _ = w.Write([]byte(`{"templateID":"tpl-bg","buildStatus":"building","public":false,"aliases":[],"names":["demo"]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	built, err := client.BuildTemplateInBackground(
		context.Background(),
		sandbox.NewTemplate().FromImage("docker.io/library/node:20"),
		"demo:v2",
		nil,
	)
	if err != nil {
		t.Fatalf("BuildTemplateInBackground: %v", err)
	}
	if statusRequested {
		t.Fatal("status should not be requested for background builds")
	}
	if built.TemplateID != "tpl-bg" || built.Status != "building" {
		t.Fatalf("built = %#v", built)
	}
}

func TestWaitForCommandPreservesCustomReadinessProbe(t *testing.T) {
	request := sandbox.NewTemplate().
		FromTemplate("tpl-node").
		SetStartCmd("npm start", sandbox.WaitForCommand("  node healthcheck.js  ")).
		Request()
	if request.StartCmd != "npm start" || request.ReadyCmd != "node healthcheck.js" {
		t.Fatalf("runtime commands = %#v", request)
	}
}

func TestTemplateFacadeBuildForwardsHighLevelOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req["name"] != "demo" {
				t.Fatalf("name = %#v", req["name"])
			}
			if got, ok := req["tags"].([]any); !ok || len(got) != 2 || got[0] != "v1" || got[1] != "latest" {
				t.Fatalf("tags = %#v", req["tags"])
			}
			if req["cpuCount"] != float64(2) || req["memoryMB"] != float64(1024) {
				t.Fatalf("resources = %#v", req)
			}
			extensions, ok := req["extensions"].(map[string]any)
			if !ok {
				t.Fatalf("extensions = %#v", req["extensions"])
			}
			if extensions["baseTemplateID"] != "tpl-base-1" || extensions["visibility"] != "personal" {
				t.Fatalf("extensions = %#v", extensions)
			}
			_, _ = w.Write([]byte(`{"templateID":"tpl-options","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v1","latest"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			if r.URL.Query().Get("logsOffset") != "0" || r.URL.Query().Get("limit") != "100" {
				t.Fatalf("query = %#v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"buildID":"build-x","templateID":"tpl-options","status":"ready","logs":[],"logEntries":[]}`))
		case r.URL.Path == "/api/v1/templates/tpl-options":
			_, _ = w.Write([]byte(`{"templateID":"tpl-options","buildStatus":"ready","public":false,"aliases":[],"names":["demo"]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-x","templateID":"tpl-options","status":"ready","image":"demo:v1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cpuCount := int32(2)
	memoryMB := int32(1024)
	client := newClient(t, server.URL)
	built, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().FromImage("docker.io/library/node:20"), "demo:v1", &sandbox.TemplateBuildOptions{
		Tags:           []string{"v1", "latest"},
		BaseTemplateID: "tpl-base-1",
		CPUCount:       &cpuCount,
		MemoryMB:       &memoryMB,
		PollInterval:   time.Millisecond,
	})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if built.TemplateID != "tpl-options" || built.Status != "ready" {
		t.Fatalf("built = %#v", built)
	}
	if strings.Join(built.Tags, "|") != "v1|latest" {
		t.Fatalf("tags = %#v", built.Tags)
	}
}

func TestTemplateManagementHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates/resolve/demo" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","public":false}`))
		case r.URL.Path == "/api/v1/templates/tpl-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case r.URL.Path == "/api/v1/templates/tpl-1/builds/build-1/status" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-1","status":"ready","logs":[],"logEntries":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	exists, err := client.TemplateExists(context.Background(), "demo")
	if err != nil || !exists {
		t.Fatalf("TemplateExists = %v, %v", exists, err)
	}
	status, err := client.GetTemplateBuildStatus(context.Background(), "tpl-1", "build-1", nil)
	if err != nil || status.Status != "ready" {
		t.Fatalf("GetTemplateBuildStatus = %#v, %v", status, err)
	}
}

func TestTemplateManagementHelpersHandleNotFoundAndForwardOptions(t *testing.T) {
	var buildStatusQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates/resolve/missing" && r.Method == http.MethodGet:
			http.NotFound(w, r)
		case r.URL.Path == "/api/v1/templates/resolve/broken" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
		case r.URL.Path == "/api/v1/templates/tpl-direct/builds/build-2/status" && r.Method == http.MethodGet:
			buildStatusQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"buildID":"build-2","templateID":"tpl-direct","status":"ready","logs":[],"logEntries":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	exists, err := client.TemplateExists(context.Background(), "missing")
	if err != nil {
		t.Fatalf("TemplateExists missing: %v", err)
	}
	if exists {
		t.Fatal("missing template should not exist")
	}
	if _, err := client.TemplateExists(context.Background(), "broken"); err == nil {
		t.Fatal("TemplateExists should propagate server errors")
	}

	logsOffset := 0
	limit := 100
	status, err := client.GetTemplateBuildStatus(context.Background(), " tpl-direct ", " build-2 ", &sandbox.TemplateBuildStatusOptions{
		LogsOffset: &logsOffset,
		Limit:      &limit,
		Level:      "info",
	})
	if err != nil {
		t.Fatalf("GetTemplateBuildStatus: %v", err)
	}
	if status.Status != "ready" {
		t.Fatalf("status = %#v", status)
	}
	if buildStatusQuery != "level=info&limit=100&logsOffset=0" && buildStatusQuery != "logsOffset=0&limit=100&level=info" && buildStatusQuery != "limit=100&logsOffset=0&level=info" && buildStatusQuery != "level=info&logsOffset=0&limit=100" && buildStatusQuery != "limit=100&level=info&logsOffset=0" && buildStatusQuery != "logsOffset=0&level=info&limit=100" {
		t.Fatalf("query = %q", buildStatusQuery)
	}
}

func TestTemplateFacadeListGetDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"templateID":"tpl-1","cpuCount":2,"memoryMB":1024,"diskSizeMB":5120,"buildStatus":"ready","public":false,"names":["demo"],"aliases":[],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0,"buildCount":1}]`))
		case r.URL.Path == "/api/v1/templates/resolve/demo":
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","public":false}`))
		case r.URL.Path == "/api/v1/templates/tpl-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case r.URL.Path == "/api/v1/templates/tpl-1" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	listed, err := client.ListTemplates(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(listed) != 1 || listed[0].TemplateID != "tpl-1" {
		t.Fatalf("listed = %#v", listed)
	}

	detail, err := client.GetTemplate(context.Background(), "demo", nil)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if detail.TemplateID != "tpl-1" {
		t.Fatalf("detail = %#v", detail)
	}

	if err := client.DeleteTemplate(context.Background(), "demo"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
}

func TestTemplateFacadeListGetDeleteForwardOptions(t *testing.T) {
	resolveCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodGet:
			if r.URL.Query().Get("visibility") != "team" || r.URL.Query().Get("teamID") != "team-1" || r.URL.Query().Get("limit") != "20" || r.URL.Query().Get("offset") != "40" {
				t.Fatalf("query = %#v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`[{"templateID":"tpl-direct","buildStatus":"ready","public":false,"names":["demo"],"aliases":[]}]`))
		case r.URL.Path == "/api/v1/templates/tpl-direct" && r.Method == http.MethodGet:
			if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("nextToken") != "build-1" {
				t.Fatalf("query = %#v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"templateID":"tpl-direct","buildStatus":"ready","public":false,"aliases":[],"names":["demo"]}`))
		case r.URL.Path == "/api/v1/templates/resolve/demo" && r.Method == http.MethodGet:
			resolveCalls++
			_, _ = w.Write([]byte(`{"templateID":"tpl-delete","public":false}`))
		case r.URL.Path == "/api/v1/templates/tpl-delete" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/v1/templates/resolve/tpl-direct" && r.Method == http.MethodGet:
			t.Fatal("direct template IDs should not be resolved")
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	listed, err := client.ListTemplates(context.Background(), &sandbox.TemplateListOptions{
		Visibility: "team",
		TeamID:     "team-1",
		Limit:      20,
		Offset:     40,
	})
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(listed) != 1 || listed[0].TemplateID != "tpl-direct" {
		t.Fatalf("listed = %#v", listed)
	}

	detail, err := client.GetTemplate(context.Background(), "tpl-direct", &sandbox.TemplateGetOptions{
		Limit:     10,
		NextToken: "build-1",
	})
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if detail.TemplateID != "tpl-direct" {
		t.Fatalf("detail = %#v", detail)
	}

	if err := client.DeleteTemplate(context.Background(), "demo"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls = %d", resolveCalls)
	}
}

func TestTemplateHelpersCompileToExpectedSteps(t *testing.T) {
	mode := 0o755
	local := false
	req := sandbox.NewTemplate().
		AptInstall([]string{"git", "curl"}, &sandbox.TemplateAptInstallOptions{NoInstallRecommends: true}).
		GitClone("https://github.com/acme/repo.git", "/app/repo", &sandbox.TemplateGitCloneOptions{
			Branch: "main",
			Depth:  1,
			User:   "root",
		}).
		MakeDir([]string{"/app/logs", "/app/cache"}, &sandbox.TemplateMakeDirOptions{
			TemplatePathOptions: sandbox.TemplatePathOptions{User: "root"},
			Mode:                &mode,
		}).
		MakeSymlink("/usr/bin/python3", "/usr/bin/python", &sandbox.TemplateMakeSymlinkOptions{
			TemplatePathOptions: sandbox.TemplatePathOptions{Force: testBoolPtr(true)},
		}).
		NpmInstall([]string{"tsx"}, &sandbox.TemplateNpmInstallOptions{G: true}).
		PipInstall([]string{"numpy"}, &sandbox.TemplatePipInstallOptions{G: &local}).
		BunInstall([]string{"prettier"}, &sandbox.TemplateBunInstallOptions{Dev: true}).
		SetWorkdir("/app/repo").
		SetUser("root").
		Request()

	if len(req.Steps) != 10 {
		t.Fatalf("steps = %#v", req.Steps)
	}
	if !strings.Contains(req.Steps[0].Args[0], "apt-get") || !strings.Contains(req.Steps[0].Args[0], "--no-install-recommends") {
		t.Fatalf("apt step = %#v", req.Steps[0])
	}
	if !strings.Contains(req.Steps[1].Args[0], "git") || !strings.Contains(req.Steps[1].Args[0], "--branch") {
		t.Fatalf("git step = %#v", req.Steps[1])
	}
	if !strings.Contains(req.Steps[2].Args[0], "mkdir") || !strings.Contains(req.Steps[3].Args[0], "mkdir") {
		t.Fatalf("workdir step = %#v", req.Steps[2])
	}
	if !strings.Contains(req.Steps[4].Args[0], "ln") || !strings.Contains(req.Steps[5].Args[0], "npm") || !strings.Contains(req.Steps[6].Args[0], "pip") || !strings.Contains(req.Steps[7].Args[0], "bun") {
		t.Fatalf("helper steps = %#v", req.Steps)
	}
	if req.Steps[8].Type != "WORKDIR" || len(req.Steps[8].Args) != 1 || req.Steps[8].Args[0] != "/app/repo" {
		t.Fatalf("workdir step = %#v", req.Steps[8])
	}
	if req.Steps[9].Type != "USER" || len(req.Steps[9].Args) != 1 || req.Steps[9].Args[0] != "root" {
		t.Fatalf("user step = %#v", req.Steps[9])
	}
}

func TestTemplateHelpersSupportSkipCacheCopyItemsRemoveAndRename(t *testing.T) {
	force := true
	req := sandbox.NewTemplate().
		SkipCache().
		CopyItems([]sandbox.TemplateCopyItem{{
			Src:       "package.json",
			Dest:      "/app/",
			FilesHash: strings.Repeat("a", 64),
		}}).
		Remove([]string{"/tmp/cache"}, &sandbox.TemplateRemoveOptions{
			TemplatePathOptions: sandbox.TemplatePathOptions{User: "root", Force: &force},
			Recursive:           true,
		}).
		Rename("/tmp/old.txt", "/tmp/new.txt", &sandbox.TemplateRenameOptions{
			TemplatePathOptions: sandbox.TemplatePathOptions{User: "root"},
		}).
		Request()

	if len(req.Steps) != 3 {
		t.Fatalf("steps = %#v", req.Steps)
	}
	if req.Steps[0].Type != "COPY" || req.Steps[0].Force == nil || !*req.Steps[0].Force {
		t.Fatalf("copy step = %#v", req.Steps[0])
	}
	if !strings.Contains(req.Steps[1].Args[0], "rm") || req.Steps[1].Force == nil || !*req.Steps[1].Force {
		t.Fatalf("remove step = %#v", req.Steps[1])
	}
	if !strings.Contains(req.Steps[2].Args[0], "mv") || req.Steps[2].Force == nil || !*req.Steps[2].Force {
		t.Fatalf("rename step = %#v", req.Steps[2])
	}
}

func TestTemplateImageHelpersAndSerialization(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(source, []byte("hello copy\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	jsonText, err := sandbox.TemplateToJSON(sandbox.NewTemplate().
		FromNodeImage("24").
		Copy(source, "/app/", nil).
		SetEnvs(map[string]string{"NODE_ENV": "production"}).
		SetStartCmd("node server.js", sandbox.WaitForPort(3000)), true)
	if err != nil {
		t.Fatalf("TemplateToJSON: %v", err)
	}
	var request map[string]any
	if err := json.Unmarshal([]byte(jsonText), &request); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if request["fromImage"] != "node:24" {
		t.Fatalf("fromImage = %#v", request["fromImage"])
	}
	steps := request["steps"].([]any)
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(steps[0].(map[string]any)["filesHash"].(string)) {
		t.Fatalf("filesHash = %#v", steps[0].(map[string]any)["filesHash"])
	}

	dockerfile, err := sandbox.TemplateToDockerfile(sandbox.NewTemplate().
		FromPythonImage("3.12").
		RunCmd("pip install numpy", nil).
		SetWorkdir("/app").
		SetUser("root"))
	if err != nil {
		t.Fatalf("TemplateToDockerfile: %v", err)
	}
	if !strings.Contains(dockerfile, "FROM python:3.12") || !strings.Contains(dockerfile, "RUN pip install numpy") || !strings.Contains(dockerfile, "WORKDIR /app") || !strings.Contains(dockerfile, "USER root") {
		t.Fatalf("dockerfile = %q", dockerfile)
	}

	registryReq := sandbox.NewTemplate().
		FromImage("example.com/acme/app:latest", map[string]any{"type": "registry", "username": "robot", "password": "secret"}).
		Request()
	if registryReq.FromImageRegistry["type"] != "registry" {
		t.Fatalf("registry config = %#v", registryReq.FromImageRegistry)
	}
	awsReq := sandbox.NewTemplate().
		FromAWSRegistry("123.dkr.ecr.us-west-2.amazonaws.com/app:latest", "AKIA", "secret", "us-west-2").
		Request()
	if awsReq.FromImageRegistry["type"] != "aws" {
		t.Fatalf("aws config = %#v", awsReq.FromImageRegistry)
	}
	gcpReq := sandbox.NewTemplate().
		FromGCPRegistry("gcr.io/acme/app:latest", map[string]any{"project_id": "acme"}).
		Request()
	if gcpReq.FromImageRegistry["type"] != "gcp" {
		t.Fatalf("gcp config = %#v", gcpReq.FromImageRegistry)
	}
}

func TestTemplateParsesDockerfilesFromInlineContentAndFilePaths(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "package.json")
	dockerfilePath := filepath.Join(tmp, "Dockerfile")
	if err := os.WriteFile(source, []byte("{\"name\":\"demo\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:20\nCOPY package.json /app/\nCMD [\"node\", \"server.js\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile Dockerfile: %v", err)
	}

	inlineTemplate, err := sandbox.NewTemplate().FromDockerfile(strings.Join([]string{
		"FROM python:3.12",
		"ENV APP_ENV=prod LOG_LEVEL=debug",
		"RUN pip install numpy",
		"WORKDIR /app",
		"USER root",
		"CMD [\"python\", \"app.py\"]",
	}, "\n"))
	if err != nil {
		t.Fatalf("FromDockerfile inline: %v", err)
	}
	inlineRequest := inlineTemplate.Request()
	if inlineRequest.FromImage != "python:3.12" {
		t.Fatalf("inline fromImage = %#v", inlineRequest.FromImage)
	}
	if len(inlineRequest.Steps) < 5 {
		t.Fatalf("inline steps = %#v", inlineRequest.Steps)
	}
	if inlineRequest.Steps[0].Type != "ENV" || len(inlineRequest.Steps[0].Args) != 2 || inlineRequest.Steps[0].Args[0] != "APP_ENV" || inlineRequest.Steps[0].Args[1] != "prod" {
		t.Fatalf("inline env step 0 = %#v", inlineRequest.Steps[0])
	}
	if inlineRequest.Steps[1].Type != "ENV" || len(inlineRequest.Steps[1].Args) != 2 || inlineRequest.Steps[1].Args[0] != "LOG_LEVEL" || inlineRequest.Steps[1].Args[1] != "debug" {
		t.Fatalf("inline env step 1 = %#v", inlineRequest.Steps[1])
	}
	if inlineRequest.Steps[2].Type != "RUN" || !strings.Contains(inlineRequest.Steps[2].Args[0], "pip install numpy") {
		t.Fatalf("inline run step = %#v", inlineRequest.Steps[2])
	}
	if inlineRequest.Steps[3].Type != "WORKDIR" || inlineRequest.Steps[3].Args[0] != "/app" {
		t.Fatalf("inline workdir step = %#v", inlineRequest.Steps[3])
	}
	if inlineRequest.Steps[4].Type != "USER" || inlineRequest.Steps[4].Args[0] != "root" {
		t.Fatalf("inline user step = %#v", inlineRequest.Steps[4])
	}
	if inlineRequest.StartCmd != "'python' 'app.py'" {
		t.Fatalf("inline startCmd = %#v", inlineRequest.StartCmd)
	}

	fileTemplate, err := sandbox.NewTemplate().FromDockerfile(dockerfilePath)
	if err != nil {
		t.Fatalf("FromDockerfile path: %v", err)
	}
	jsonText, err := sandbox.TemplateToJSON(fileTemplate, true)
	if err != nil {
		t.Fatalf("TemplateToJSON: %v", err)
	}
	var request map[string]any
	if err := json.Unmarshal([]byte(jsonText), &request); err != nil {
		t.Fatalf("Unmarshal file request: %v", err)
	}
	if request["fromImage"] != "node:20" {
		t.Fatalf("file fromImage = %#v", request["fromImage"])
	}
	steps := request["steps"].([]any)
	first := steps[0].(map[string]any)
	if first["type"] != "COPY" {
		t.Fatalf("file copy step = %#v", first)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(first["filesHash"].(string)) {
		t.Fatalf("file filesHash = %#v", first["filesHash"])
	}
	if request["startCmd"] != "'node' 'server.js'" {
		t.Fatalf("file startCmd = %#v", request["startCmd"])
	}
}

func TestTemplateRejectsUnsupportedDockerfileInstructions(t *testing.T) {
	if _, err := sandbox.NewTemplate().FromDockerfile("FROM node:20\nENTRYPOINT [\"node\"]\n"); err == nil || !strings.Contains(err.Error(), "unsupported Dockerfile instruction: ENTRYPOINT") {
		t.Fatalf("err = %v", err)
	}
}

func TestTemplateSupportsRunCmdUserAndCopyTarOptions(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "hello.txt")
	link := filepath.Join(tmp, "hello-link.txt")
	if err := os.WriteFile(source, []byte("hello copy\n"), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	if err := os.Symlink(source, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	runRequest := sandbox.NewTemplate().
		RunCmd("apt-get install vim", &sandbox.TemplateCommandOptions{User: "root"}).
		Request()
	if len(runRequest.Steps) != 1 || !strings.Contains(runRequest.Steps[0].Args[0], "su -s /bin/sh") {
		t.Fatalf("run request = %#v", runRequest.Steps)
	}

	defaultJSON, err := sandbox.TemplateToJSON(sandbox.NewTemplate().FromBaseImage().Copy(link, "/app/", nil), true)
	if err != nil {
		t.Fatalf("TemplateToJSON default: %v", err)
	}
	mode := 0o600
	modeJSON, err := sandbox.TemplateToJSON(sandbox.NewTemplate().FromBaseImage().Copy(link, "/app/", &sandbox.TemplateCopyOptions{Mode: &mode}), true)
	if err != nil {
		t.Fatalf("TemplateToJSON mode: %v", err)
	}
	resolvedJSON, err := sandbox.TemplateToJSON(sandbox.NewTemplate().FromBaseImage().Copy(link, "/app/", &sandbox.TemplateCopyOptions{ResolveSymlinks: true}), true)
	if err != nil {
		t.Fatalf("TemplateToJSON resolveSymlinks: %v", err)
	}

	extractHash := func(raw string) string {
		var request map[string]any
		if err := json.Unmarshal([]byte(raw), &request); err != nil {
			t.Fatalf("Unmarshal request: %v", err)
		}
		steps := request["steps"].([]any)
		return steps[0].(map[string]any)["filesHash"].(string)
	}

	defaultHash := extractHash(defaultJSON)
	modeHash := extractHash(modeJSON)
	resolvedHash := extractHash(resolvedJSON)
	if defaultHash == modeHash {
		t.Fatalf("expected mode hash to differ: %s", defaultHash)
	}
	if defaultHash == resolvedHash {
		t.Fatalf("expected resolved symlink hash to differ: %s", defaultHash)
	}
}

func TestFilesystemWriteHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-files",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/file":
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/runtime/files/batch":
			_, _ = w.Write([]byte(`{"files":[{"path":"/tmp/a.txt","bytes_written":1},{"path":"/tmp/b.txt","bytes_written":2}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	files, err := created.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	written, err := files.Write(context.Background(), "/tmp/hello.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written.Path != "/tmp/hello.txt" || written.BytesWritten != 5 {
		t.Fatalf("written = %#v", written)
	}
	batch, err := files.WriteFiles(context.Background(), []cmd.WriteFileEntry{
		{Path: "/tmp/a.txt", Content: "a"},
		{Path: "/tmp/b.txt", Content: "bb"},
	})
	if err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	if len(batch) != 2 || batch[0].BytesWritten != 1 || batch[1].BytesWritten != 2 {
		t.Fatalf("batch = %#v", batch)
	}
}

func TestFacadeFilesystemGitAndProxyHelpers(t *testing.T) {
	runCalls := make([]map[string]any, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sandboxes":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-ops",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/Stat":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode stat request: %v", err)
			}
			if req["path"] == "/tmp/missing" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entry":{"name":"a.txt","type":"FILE_TYPE_FILE","path":"` + req["path"].(string) + `"}}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/ListDir":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode list request: %v", err)
			}
			if req["path"] != "/tmp" || req["depth"] != float64(1) {
				t.Fatalf("list request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entries":[{"name":"a.txt","type":"FILE_TYPE_FILE","path":"/tmp/a.txt"}]}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/MakeDir":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode mkdir request: %v", err)
			}
			if req["path"] != "/tmp/new" {
				t.Fatalf("mkdir request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entry":{"path":"/tmp/new","type":"FILE_TYPE_DIRECTORY"}}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/Remove":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode remove request: %v", err)
			}
			if req["path"] != "/tmp/old" {
				t.Fatalf("remove request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/Move":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode move request: %v", err)
			}
			if req["source"] != "/tmp/a.txt" || req["destination"] != "/tmp/b.txt" {
				t.Fatalf("move request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entry":{"path":"/tmp/b.txt","type":"FILE_TYPE_FILE"}}`))
		case r.URL.Path == "/runtime/filesystem.Filesystem/WatchDir":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode watch request: %v", err)
			}
			if req["path"] != "/tmp" || req["recursive"] != true {
				t.Fatalf("watch request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/connect+json")
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"filesystem": map[string]any{"name": "a.txt", "type": "EVENT_TYPE_WRITE"},
			}))
		case r.URL.Path == "/runtime/run":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode run request: %v", err)
			}
			runCalls = append(runCalls, req)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"stdout":"ok\n","stderr":"","exit_code":0,"duration_ms":1}`))
		case r.URL.Path == "/runtime/proxy/8080/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("proxied"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	files, err := created.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	exists, err := files.Exists(context.Background(), "/tmp/missing")
	if err != nil {
		t.Fatalf("Exists missing: %v", err)
	}
	if exists {
		t.Fatal("missing path should not exist")
	}
	exists, err = files.Exists(context.Background(), "/tmp/a.txt")
	if err != nil || !exists {
		t.Fatalf("Exists = %v, %v", exists, err)
	}
	info, err := files.GetInfo(context.Background(), "/tmp/a.txt")
	if err != nil || info.Path != "/tmp/a.txt" {
		t.Fatalf("GetInfo = %#v, %v", info, err)
	}
	depth := 1
	entries, err := files.List(context.Background(), "/tmp", &depth)
	if err != nil || len(entries) != 1 || entries[0].Path != "/tmp/a.txt" {
		t.Fatalf("List = %#v, %v", entries, err)
	}
	createdDir, err := files.MakeDir(context.Background(), "/tmp/new")
	if err != nil || !createdDir {
		t.Fatalf("MakeDir = %v, %v", createdDir, err)
	}
	if err := files.Remove(context.Background(), "/tmp/old"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	renamed, err := files.Rename(context.Background(), "/tmp/a.txt", "/tmp/b.txt")
	if err != nil || renamed.Path != "/tmp/b.txt" {
		t.Fatalf("Rename = %#v, %v", renamed, err)
	}
	recursive := true
	watch, err := files.WatchDir(context.Background(), "/tmp", &recursive)
	if err != nil {
		t.Fatalf("WatchDir: %v", err)
	}
	frame, err := watch.Next()
	if err != nil {
		t.Fatalf("watch.Next: %v", err)
	}
	if frame.Filesystem == nil || frame.Filesystem.Name != "a.txt" || frame.Filesystem.Type != cmd.EventType("EVENT_TYPE_WRITE") {
		t.Fatalf("frame = %#v", frame)
	}
	_ = watch.Close()

	git, err := created.Git()
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	if _, err := git.Pull(context.Background(), "/workspace/repo", &sandbox.GitCommandOptions{
		Envs:    map[string]string{"A": "1"},
		Timeout: intPtr(5),
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := git.Checkout(context.Background(), "main", "/workspace/repo", nil); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if _, err := git.Status(context.Background(), "/workspace/repo", nil); err != nil {
		t.Fatalf("Status: %v", err)
	}

	resp, err := created.Proxy(context.Background(), &cmd.ProxyRequest{
		Method: http.MethodGet,
		Port:   8080,
		Path:   "/health",
	})
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "proxied" {
		t.Fatalf("body = %q", string(body))
	}

	if len(runCalls) != 3 {
		t.Fatalf("runCalls = %#v", runCalls)
	}
	if strings.Join(anySliceToStrings(runCalls[0]["args"].([]any)), "|") != "pull" || runCalls[0]["cwd"] != "/workspace/repo" {
		t.Fatalf("pull = %#v", runCalls[0])
	}
	if env, _ := runCalls[0]["env"].(map[string]any); env["A"] != "1" || runCalls[0]["timeout"] != float64(5) {
		t.Fatalf("pull env = %#v", runCalls[0])
	}
	if strings.Join(anySliceToStrings(runCalls[1]["args"].([]any)), "|") != "checkout|main" {
		t.Fatalf("checkout = %#v", runCalls[1])
	}
	if strings.Join(anySliceToStrings(runCalls[2]["args"].([]any)), "|") != "status" {
		t.Fatalf("status = %#v", runCalls[2])
	}
}

func TestFacadeCommandAndPTYHandles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sandboxes":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"templateID":"base",
				"sandboxID":"sb-handle",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"http://` + r.Host + `/runtime",
				"status":"running",
				"state":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/runtime/process.Process/Start":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode start request: %v", err)
			}
			w.Header().Set("Content-Type", "application/connect+json")
			if req["pty"] != nil {
				_, _ = w.Write(connectFrameJSON(t, map[string]any{
					"event": map[string]any{
						"start": map[string]any{"pid": 99, "cmdId": "cmd-pty"},
					},
				}))
				_, _ = w.Write(connectFrameJSON(t, map[string]any{
					"event": map[string]any{
						"data": map[string]any{"pty": base64.StdEncoding.EncodeToString([]byte("shell$ "))},
					},
				}))
				_, _ = w.Write(connectFrameJSON(t, map[string]any{
					"event": map[string]any{
						"end": map[string]any{"exited": true, "status": "exited", "error": nil},
					},
				}))
				return
			}
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"event": map[string]any{
					"start": map[string]any{"pid": 42, "cmdId": "cmd-1"},
				},
			}))
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"event": map[string]any{
					"data": map[string]any{"stdout": base64.StdEncoding.EncodeToString([]byte("hello\n"))},
				},
			}))
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"event": map[string]any{
					"end": map[string]any{"exited": true, "status": "exited", "error": nil},
				},
			}))
		case r.URL.Path == "/runtime/process.Process/SendInput":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode send input request: %v", err)
			}
			input := req["input"].(map[string]any)
			if got, _ := input["stdin"].(string); got != "" && got != base64.StdEncoding.EncodeToString([]byte("ping\n")) {
				t.Fatalf("stdin = %q", got)
			}
			if got, _ := input["pty"].(string); got != "" && got != base64.StdEncoding.EncodeToString([]byte("ls\n")) {
				t.Fatalf("pty = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/runtime/process.Process/CloseStdin":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/runtime/process.Process/Connect":
			w.Header().Set("Content-Type", "application/connect+json")
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"event": map[string]any{
					"start": map[string]any{"pid": 99},
				},
			}))
			_, _ = w.Write(connectFrameJSON(t, map[string]any{
				"event": map[string]any{
					"data": map[string]any{"stdout": base64.StdEncoding.EncodeToString([]byte("connected$ "))},
				},
			}))
		case r.URL.Path == "/runtime/process.Process/Update":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode update request: %v", err)
			}
			size := req["pty"].(map[string]any)["size"].(map[string]any)
			if req["process"].(map[string]any)["pid"] != float64(99) || size["cols"] != float64(100) || size["rows"] != float64(40) {
				t.Fatalf("update request = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/runtime/process.Process/SendSignal":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode signal request: %v", err)
			}
			if req["process"].(map[string]any)["pid"] == float64(404) {
				http.NotFound(w, r)
				return
			}
			if req["process"].(map[string]any)["pid"] == float64(405) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"kill failed: ESRCH: No such process"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/runtime/process.Process/GetResult":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"exitCode":0,"stdout":"hello\n","stderr":"","startedAtUnix":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(t, server.URL)
	created, err := client.Create(context.Background(), "base", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	commands, err := created.Commands()
	if err != nil {
		t.Fatalf("Commands: %v", err)
	}
	handle, err := commands.Start(context.Background(), "cat", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := handle.SendStdin(context.Background(), "ping\n"); err != nil {
		t.Fatalf("handle.SendStdin: %v", err)
	}
	if err := handle.CloseStdin(context.Background()); err != nil {
		t.Fatalf("handle.CloseStdin: %v", err)
	}
	waited, err := handle.Wait(context.Background())
	if err != nil {
		t.Fatalf("handle.Wait: %v", err)
	}
	if waited.ExitCode != 0 || strings.TrimSpace(waited.Stdout) != "hello" {
		t.Fatalf("waited = %#v", waited)
	}

	pty, err := created.Pty()
	if err != nil {
		t.Fatalf("Pty: %v", err)
	}
	ptyHandle, err := pty.Create(context.Background(), "bash", nil)
	if err != nil {
		t.Fatalf("pty.Create: %v", err)
	}
	if err := ptyHandle.SendStdin(context.Background(), "ls\n"); err != nil {
		t.Fatalf("ptyHandle.SendStdin: %v", err)
	}
	ptyWaited, err := ptyHandle.Wait(context.Background())
	if err != nil {
		t.Fatalf("ptyHandle.Wait: %v", err)
	}
	if ptyWaited.PTY != "shell$ " {
		t.Fatalf("ptyWaited = %#v", ptyWaited)
	}
	connectedHandle, err := pty.Connect(context.Background(), 99)
	if err != nil {
		t.Fatalf("pty.Connect: %v", err)
	}
	connectedWaited, err := connectedHandle.Wait(context.Background())
	if err != nil {
		t.Fatalf("connectedHandle.Wait: %v", err)
	}
	if connectedWaited.PTY != "connected$ " {
		t.Fatalf("connectedWaited = %#v", connectedWaited)
	}
	if err := pty.Resize(context.Background(), 99, cmd.PtySize{Cols: 100, Rows: 40}); err != nil {
		t.Fatalf("pty.Resize: %v", err)
	}
	killed, err := pty.Kill(context.Background(), 99)
	if err != nil || !killed {
		t.Fatalf("pty.Kill = %v, %v", killed, err)
	}
	missing, err := pty.Kill(context.Background(), 404)
	if err != nil || missing {
		t.Fatalf("pty.Kill missing = %v, %v", missing, err)
	}
	esrchMissing, err := pty.Kill(context.Background(), 405)
	if err != nil || esrchMissing {
		t.Fatalf("pty.Kill ESRCH missing = %v, %v", esrchMissing, err)
	}
}

func TestTemplateBuildAutoUploadsLocalCopySources(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(source, []byte("hello copy\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	uploads := 0
	var uploadBody []byte
	var copiedStep map[string]any
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-copy","buildID":"build-copy","public":false,"names":["demo"],"tags":["auto-copy"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/files/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"present":false,"url":"` + serverURL + `/upload/file.tar"}`))
		case r.URL.Path == "/upload/file.tar" && r.Method == http.MethodPut:
			uploads++
			uploadBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&copiedStep); err != nil {
				t.Fatalf("decode create build: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"buildID":"build-copy","templateID":"tpl-copy","status":"ready","logs":[],"logEntries":[]}`))
		case r.URL.Path == "/api/v1/templates/tpl-copy":
			_, _ = w.Write([]byte(`{"templateID":"tpl-copy","buildStatus":"ready","public":false,"aliases":[],"names":["demo"]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-copy","templateID":"tpl-copy","status":"ready"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := newClient(t, server.URL)
	_, err := client.BuildTemplate(context.Background(), sandbox.NewTemplate().
		FromImage("docker.io/library/alpine:3.20").
		Copy(source, "/app/", nil),
		"demo:auto-copy",
		&sandbox.TemplateBuildOptions{
			PollInterval: 0,
		},
	)
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if uploads != 1 {
		t.Fatalf("uploads = %d", uploads)
	}
	header := uploadBody
	if len(header) > 2 {
		header = header[:2]
	}
	if len(uploadBody) < 2 || !bytes.Equal(uploadBody[:2], []byte{0x1f, 0x8b}) {
		t.Fatalf("upload header = %v", header)
	}
	steps := copiedStep["steps"].([]any)
	first := steps[0].(map[string]any)
	if first["args"].([]any)[0].(string) != "hello.txt" {
		t.Fatalf("copy src = %#v", first["args"])
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(first["filesHash"].(string)) {
		t.Fatalf("filesHash = %#v", first["filesHash"])
	}
}

func testBoolPtr(v bool) *bool {
	return &v
}

func anySliceToStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.(string))
	}
	return out
}
