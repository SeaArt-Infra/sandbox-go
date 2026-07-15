package tests

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaArt-Infra/sandbox-go/build"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func TestDirectBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/build" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace header = %q", got)
		}
		if got := r.Header.Get("X-Project-ID"); got != "project-1" {
			t.Fatalf("project header = %q", got)
		}

		var req build.DirectBuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Project != "proj" || req.Image != "app" || req.Tag != "v1" {
			t.Fatalf("request = %#v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"imageFullName":"example-image:v1"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(
		server.URL,
		"unit-auth-value",
		core.WithProjectID("project-1"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.DirectBuild(context.Background(), &build.DirectBuildRequest{
		Project:    "proj",
		Image:      "app",
		Tag:        "v1",
		Dockerfile: "FROM alpine:3.20",
	})
	if err != nil {
		t.Fatalf("DirectBuild: %v", err)
	}
	if resp.TemplateID != "tpl-1" || resp.BuildID != "build-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateTemplateUsesGatewayAuthOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req build.TemplateCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace = %q", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "" {
			t.Fatalf("unexpected user = %q", got)
		}
		if got := r.Header.Get("X-Role"); got != "" {
			t.Fatalf("unexpected role = %q", got)
		}
		if got := r.Header.Get("X-User-Email"); got != "" {
			t.Fatalf("unexpected email = %q", got)
		}
		if req.Name != "demo" {
			t.Fatalf("request = %#v", req)
		}
		if len(req.Tags) != 1 || req.Tags[0] != "v1" {
			t.Fatalf("tags = %#v", req.Tags)
		}
		if req.CPUCount == nil || *req.CPUCount != 2 {
			t.Fatalf("cpuCount = %#v", req.CPUCount)
		}
		if req.MemoryMB == nil || *req.MemoryMB != 1024 {
			t.Fatalf("memoryMB = %#v", req.MemoryMB)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"public":false,
			"names":["user-1/demo"],
			"tags":["v1"],
			"aliases":["demo"]
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name:     "demo",
		Tags:     []string{"v1"},
		CPUCount: int32Ptr(2),
		MemoryMB: int32Ptr(1024),
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if len(resp.Names) != 1 || resp.Names[0] != "user-1/demo" {
		t.Fatalf("names = %#v", resp.Names)
	}
}

func TestGetTemplateDecodesFullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("nextToken") != "build-1" {
			t.Fatalf("query = %#v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-2",
			"buildStatus":"ready",
			"cpuCount":2,
			"memoryMB":1024,
			"diskSizeMB":5120,
			"public":false,
			"names":["user-1/demo"],
			"aliases":["demo"],
			"createdBy":{"id":"user-1","email":"user@example.com"},
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:01:00Z",
			"lastSpawnedAt":"2026-01-01T00:02:00Z",
			"spawnCount":3,
			"buildCount":4,
			"envdVersion":"sandbox-builder-v1",
			"visibility":"personal",
			"image":"harbor.example/demo:latest",
			"storageType":"nfs",
			"startCmd":"npm start",
			"readyCmd":"test -f /tmp/ready",
			"cloudsinkURL":"https://cloudsink.internal",
			"builds":[
				{
					"buildID":"build-2",
					"status":"ready",
					"createdAt":"2026-01-01T00:00:00Z",
					"updatedAt":"2026-01-01T00:02:00Z",
					"finishedAt":"2026-01-01T00:02:00Z",
					"cpuCount":2,
					"memoryMB":1024,
					"diskSizeMB":5120,
					"envdVersion":"sandbox-builder-v1"
				}
			],
			"nextToken":"build-next"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetTemplate(context.Background(), "tpl-1", &build.GetTemplateParams{
		Limit:     10,
		NextToken: "build-1",
	})
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if resp.TemplateID != "tpl-1" {
		t.Fatalf("response = %#v", resp)
	}
	if resp.BuildID != "build-2" || resp.BuildStatus != "ready" {
		t.Fatalf("build fields = %#v", resp)
	}
	if resp.CPUCount == nil || *resp.CPUCount != 2 || resp.MemoryMB == nil || *resp.MemoryMB != 1024 {
		t.Fatalf("resource fields = %#v", resp)
	}
	if resp.CreatedBy == nil || resp.CreatedBy.Email != "user@example.com" {
		t.Fatalf("createdBy = %#v", resp.CreatedBy)
	}
	if resp.Visibility != "personal" || resp.StorageType != "nfs" {
		t.Fatalf("template fields = %#v", resp)
	}
	if resp.StartCmd != "npm start" || resp.ReadyCmd != "test -f /tmp/ready" {
		t.Fatalf("cmd fields = %#v", resp)
	}
	if resp.CloudsinkURL != "https://cloudsink.internal" {
		t.Fatalf("cloudsinkURL = %q", resp.CloudsinkURL)
	}
	if len(resp.Builds) != 1 || resp.Builds[0].Status != "ready" {
		t.Fatalf("builds = %#v", resp.Builds)
	}
	if resp.NextToken != "build-next" || resp.Builds[0].MemoryMB != 1024 {
		t.Fatalf("unexpected fields = %#v", resp)
	}
}

func TestListTemplatesEncodesQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("visibility"); got != "team" {
			t.Fatalf("visibility = %q", got)
		}
		if got := q.Get("teamID"); got != "ns-1" {
			t.Fatalf("teamID = %q", got)
		}
		if got := q.Get("limit"); got != "20" {
			t.Fatalf("limit = %q", got)
		}
		if got := q.Get("offset"); got != "40" {
			t.Fatalf("offset = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"buildStatus":"ready",
			"public":false,
			"names":["user-1/demo"],
			"aliases":["demo"],
			"createdBy":{"id":"user-1"}
		}]`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.ListTemplates(context.Background(), &build.ListTemplatesParams{
		Visibility: "team",
		TeamID:     "ns-1",
		Limit:      20,
		Offset:     40,
	})
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(resp) != 1 || resp[0].TemplateID != "tpl-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestTemplateValidationRejectsUnsupportedPublicExtensions(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name: "demo",
		Extensions: &build.PublicTemplateExtensions{
			Seacloud: &build.PublicSeacloudTemplateExtensions{
				Visibility: "official",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "extensions.seacloud.visibility=official is not supported by the public SDK") {
		t.Fatalf("CreateTemplate error = %v", err)
	}

	_, err = service.UpdateTemplate(context.Background(), "tpl-1", &build.TemplateUpdateRequest{
		Extensions: &build.PublicTemplateExtensions{
			Seacloud: &build.PublicSeacloudTemplateExtensions{
				Visibility: "official",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "extensions.seacloud.visibility=official is not supported by the public SDK") {
		t.Fatalf("UpdateTemplate error = %v", err)
	}
}

func TestGetTemplateByAliasUsesAliasEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/templates/aliases/demo-alias" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateID":"tpl-1","public":false}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetTemplateByAlias(context.Background(), "demo-alias")
	if err != nil {
		t.Fatalf("GetTemplateByAlias: %v", err)
	}
	if resp.TemplateID != "tpl-1" {
		t.Fatalf("templateID = %q", resp.TemplateID)
	}
}

func TestResolveTemplateRefUsesResolveEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/templates/resolve/base" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateID":"tpl-base","public":true}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.ResolveTemplateRef(context.Background(), "base")
	if err != nil {
		t.Fatalf("ResolveTemplateRef: %v", err)
	}
	if resp.TemplateID != "tpl-base" {
		t.Fatalf("templateID = %q", resp.TemplateID)
	}
}

func TestTemplateBuildBuilderRequest(t *testing.T) {
	request := build.NewTemplateBuildBuilder().
		FromImage("docker.io/library/node:20").
		FromImageRegistry(map[string]any{
			"type":     "registry",
			"username": "robot",
			"password": "secret",
		}).
		Force(true).
		Copy("package.json", "/app/package.json", strings.Repeat("a", 64), &build.CopyStepOptions{Force: boolPtr(true)}).
		Run("npm ci", nil).
		EnvMap(map[string]string{
			"NODE_ENV": "production",
			"PORT":     "3000",
		}).
		Workdir("/app", nil).
		User("node", nil).
		StartCmd("npm start").
		ReadyCmd("test-ready-command").
		FilesHash(strings.Repeat("b", 64)).
		Request()

	if request.FromImage != "docker.io/library/node:20" {
		t.Fatalf("request = %#v", request)
	}
	if request.Force == nil || !*request.Force {
		t.Fatalf("force = %#v", request.Force)
	}
	if request.FromImageRegistry["username"] != "robot" {
		t.Fatalf("registry = %#v", request.FromImageRegistry)
	}
	if len(request.Steps) != 5 {
		t.Fatalf("steps = %#v", request.Steps)
	}
	if request.Steps[0].Type != "COPY" || request.Steps[0].Force == nil || !*request.Steps[0].Force {
		t.Fatalf("copy step = %#v", request.Steps[0])
	}
	if request.Steps[2].Type != "ENV" || len(request.Steps[2].Args) != 4 {
		t.Fatalf("env step = %#v", request.Steps[2])
	}
	if got := strings.Join(request.Steps[2].Args, ","); got != "NODE_ENV,production,PORT,3000" {
		t.Fatalf("env args = %q", got)
	}
	if request.StartCmd != "npm start" || request.ReadyCmd != "test-ready-command" {
		t.Fatalf("commands = %#v", request)
	}
}

func TestTemplateBuildBuilderRequestIsDefensiveCopy(t *testing.T) {
	builder := build.NewTemplateBuildBuilder().
		FromImage("docker.io/library/alpine:3.20").
		Copy("src", "/dst", strings.Repeat("a", 64), nil).
		Env("NODE_ENV", "production")

	request := builder.Request()
	request.FromImage = "changed"
	request.Steps[0].Args[0] = "mutated"

	next := builder.Request()
	if next.FromImage != "docker.io/library/alpine:3.20" {
		t.Fatalf("fromImage = %q", next.FromImage)
	}
	if next.Steps[0].Args[0] != "src" {
		t.Fatalf("copy args = %#v", next.Steps[0].Args)
	}
}

func TestCreateBuildReturnsRawEmptyObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/templates/tpl-1/builds/build-abc" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var req build.BuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.FromTemplate != "base" {
			t.Fatalf("fromTemplate = %q", req.FromTemplate)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", "build-abc", &build.BuildRequest{FromTemplate: "base"})
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
}

func TestCreateBuildUsesEmptyResponseAndEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/templates/tpl-1/builds/build-empty" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty body, got %q", string(body))
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Fatalf("unexpected content-type = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", "build-empty", nil)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
}

func TestCreateBuildEncodesSupportedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/templates/tpl-1/builds/build-encoded" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req build.BuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.FromImage != "docker.io/library/node:20" || req.FilesHash != strings.Repeat("a", 64) {
			t.Fatalf("request = %#v", req)
		}
		if req.FromImageRegistry["type"] != "registry" || req.FromImageRegistry["username"] != "robot" {
			t.Fatalf("registry config = %#v", req.FromImageRegistry)
		}
		if req.StartCmd != "npm start" || req.ReadyCmd != "test-ready-command" {
			t.Fatalf("commands = %#v", req)
		}
		if len(req.Steps) != 3 || req.Steps[0].Type != "COPY" || req.Steps[0].FilesHash != strings.Repeat("a", 64) {
			t.Fatalf("steps = %#v", req.Steps)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", "build-encoded", &build.BuildRequest{
		FromImage:         "docker.io/library/node:20",
		FromImageRegistry: map[string]any{"type": "registry", "username": "robot", "password": "secret"},
		FilesHash:         strings.Repeat("a", 64),
		Steps: []build.BuildStep{
			{Type: "COPY", FilesHash: strings.Repeat("a", 64), Args: []string{"package.json", "/app/package.json"}},
			{Type: "RUN", Args: []string{"npm install"}},
			{Type: "ENV", Args: []string{"NODE_ENV", "production"}},
		},
		StartCmd: "npm start",
		ReadyCmd: "test-ready-command",
	})
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
}

func TestCreateBuildRejectsUnsupportedFields(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.CreateBuild(context.Background(), "tpl-1", "build-unsupported", &build.BuildRequest{
		FromImageRegistry: map[string]any{"type": "registry"},
	})
	if err == nil || !strings.Contains(err.Error(), "fromImageRegistry") {
		t.Fatalf("CreateBuild error = %v", err)
	}
}

func TestBuildValidationErrors(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	testCases := []struct {
		name string
		fn   func() error
		want string
	}{
		{
			name: "invalid build id",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", "Build-Uppercase", nil)
				return err
			},
			want: "buildID must be",
		},
		{
			name: "invalid files hash",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", "build-test", &build.BuildRequest{
					FilesHash: "abc",
				})
				return err
			},
			want: "filesHash must be",
		},
		{
			name: "missing step type",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", "build-test", &build.BuildRequest{
					Steps: []build.BuildStep{{FilesHash: strings.Repeat("a", 64)}},
				})
				return err
			},
			want: "steps[0].type is required",
		},
		{
			name: "copy args missing destination",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", "build-test", &build.BuildRequest{
					Steps: []build.BuildStep{{Type: "COPY", FilesHash: strings.Repeat("a", 64), Args: []string{"x"}}},
				})
				return err
			},
			want: "steps[0].args must include src and dest for COPY",
		},
		{
			name: "env pairs invalid",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", "build-test", &build.BuildRequest{
					Steps: []build.BuildStep{{Type: "ENV", Args: []string{"NODE_ENV"}}},
				})
				return err
			},
			want: "steps[0].args must contain ENV key/value pairs",
		},
		{
			name: "invalid status limit",
			fn: func() error {
				_, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", &build.BuildStatusParams{Limit: intPtr(101)})
				return err
			},
			want: "build status limit",
		},
		{
			name: "invalid logs direction",
			fn: func() error {
				_, err := service.GetBuildLogs(context.Background(), "tpl-1", "build-1", &build.BuildLogsParams{Direction: "sideways"})
				return err
			},
			want: "build logs direction",
		},
		{
			name: "invalid template list offset",
			fn: func() error {
				_, err := service.ListTemplates(context.Background(), &build.ListTemplatesParams{Offset: -1})
				return err
			},
			want: "template list offset",
		},
		{
			name: "invalid template detail limit",
			fn: func() error {
				_, err := service.GetTemplate(context.Background(), "tpl-1", &build.GetTemplateParams{Limit: 101})
				return err
			},
			want: "template build history limit",
		},
		{
			name: "empty alias",
			fn: func() error {
				_, err := service.GetTemplateByAlias(context.Background(), " ")
				return err
			},
			want: "alias is required",
		},
		{
			name: "invalid hash query",
			fn: func() error {
				_, err := service.GetBuildFile(context.Background(), "tpl-1", "bad")
				return err
			},
			want: "hash must be",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestBuildBoundaryValuesAreAccepted(t *testing.T) {
	buildID := "build-" + strings.Repeat("a", 57)
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates":
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/api/v1/templates/aliases/demo":
			_, _ = w.Write([]byte(`{"templateID":"tpl-1"}`))
		case r.URL.Path == "/api/v1/templates/resolve/base":
			_, _ = w.Write([]byte(`{"templateID":"tpl-base"}`))
		case r.URL.Path == "/api/v1/templates/tpl-1":
			_, _ = w.Write([]byte(`{"templateID":"tpl-1"}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"buildID":"b","templateID":"tpl-1","status":"building","logs":[],"logEntries":[]}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		case strings.Contains(r.URL.Path, "/files/"):
			_, _ = w.Write([]byte(`{"present":true}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := service.ListTemplates(context.Background(), &build.ListTemplatesParams{Limit: 100, Offset: 0}); err != nil {
		t.Fatalf("ListTemplates boundary: %v", err)
	}
	if _, err := service.GetTemplateByAlias(context.Background(), "demo"); err != nil {
		t.Fatalf("GetTemplateByAlias boundary: %v", err)
	}
	if _, err := service.ResolveTemplateRef(context.Background(), "base"); err != nil {
		t.Fatalf("ResolveTemplateRef boundary: %v", err)
	}
	if _, err := service.GetTemplate(context.Background(), "tpl-1", &build.GetTemplateParams{Limit: 100}); err != nil {
		t.Fatalf("GetTemplate boundary: %v", err)
	}
	if _, err := service.CreateBuild(context.Background(), "tpl-1", buildID, nil); err != nil {
		t.Fatalf("CreateBuild boundary: %v", err)
	}
	logsOffset := 0
	limit := 100
	if _, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", &build.BuildStatusParams{LogsOffset: &logsOffset, Limit: &limit}); err != nil {
		t.Fatalf("GetBuildStatus boundary: %v", err)
	}
	cursor := int64(0)
	if _, err := service.GetBuildLogs(context.Background(), "tpl-1", "build-1", &build.BuildLogsParams{Cursor: &cursor, Limit: &limit, Direction: "backward", Source: "temporary"}); err != nil {
		t.Fatalf("GetBuildLogs boundary: %v", err)
	}
	if _, err := service.GetBuildFile(context.Background(), "tpl-1", strings.Repeat("a", 64)); err != nil {
		t.Fatalf("GetBuildFile boundary: %v", err)
	}
	if len(calls) != 8 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestBuildEmptyIdentifiersValidation(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	testCases := []struct {
		name string
		fn   func() error
	}{
		{"empty get template", func() error { _, err := service.GetTemplate(context.Background(), " ", nil); return err }},
		{"empty update template", func() error { _, err := service.UpdateTemplate(context.Background(), " ", nil); return err }},
		{"empty delete template", func() error { return service.DeleteTemplate(context.Background(), " ") }},
		{"empty alias", func() error { _, err := service.GetTemplateByAlias(context.Background(), " "); return err }},
		{"empty ref", func() error { _, err := service.ResolveTemplateRef(context.Background(), " "); return err }},
		{"empty create build template", func() error { _, err := service.CreateBuild(context.Background(), " ", "build-1", nil); return err }},
		{"empty create build id", func() error { _, err := service.CreateBuild(context.Background(), "tpl-1", " ", nil); return err }},
		{"empty get build template", func() error { _, err := service.GetBuild(context.Background(), " ", "build-1"); return err }},
		{"empty get build id", func() error { _, err := service.GetBuild(context.Background(), "tpl-1", " "); return err }},
		{"empty get build status template", func() error { _, err := service.GetBuildStatus(context.Background(), " ", "build-1", nil); return err }},
		{"empty get build logs id", func() error { _, err := service.GetBuildLogs(context.Background(), "tpl-1", " ", nil); return err }},
		{"empty list builds", func() error { _, err := service.ListBuilds(context.Background(), " "); return err }},
		{"empty build file template", func() error {
			_, err := service.GetBuildFile(context.Background(), " ", strings.Repeat("a", 64))
			return err
		}},
		{"empty build file hash", func() error { _, err := service.GetBuildFile(context.Background(), "tpl-1", " "); return err }},
		{"empty rollback template", func() error {
			_, err := service.RollbackTemplate(context.Background(), " ", &build.RollbackRequest{BuildID: "build-1"})
			return err
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestGetBuildStatusAllowsAnonymousPolling(t *testing.T) {
	logsOffset := 5
	limit := 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace header = %q", got)
		}
		q := r.URL.Query()
		if got := q.Get("logsOffset"); got != "5" {
			t.Fatalf("logsOffset = %q", got)
		}
		if got := q.Get("limit"); got != "10" {
			t.Fatalf("limit = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"buildID":"build-1",
			"templateID":"tpl-1",
			"status":"building",
			"logs":["raw-line"],
			"logEntries":[
				{
					"timestamp":"2026-01-01T00:00:00Z",
					"level":"info",
					"step":"build",
					"message":"building image"
				}
			],
			"reason":null,
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:00:01Z"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", &build.BuildStatusParams{
		LogsOffset: &logsOffset,
		Limit:      &limit,
	})
	if err != nil {
		t.Fatalf("GetBuildStatus: %v", err)
	}
	if len(resp.LogEntries) != 1 || resp.LogEntries[0].Message != "building image" || len(resp.Logs) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestBuildListGetAndLogsEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates/tpl-1/builds":
			_, _ = w.Write([]byte(`{
				"builds":[
					{
						"buildID":"build-1",
						"templateID":"tpl-1",
						"status":"ready",
						"image":"example-image:v1",
						"errorMessage":"",
						"createdAt":"2026-01-01T00:00:00Z",
						"updatedAt":"2026-01-01T00:02:00Z",
						"finishedAt":"2026-01-01T00:02:00Z"
					}
				],
				"total":1
			}`))
		case r.URL.Path == "/api/v1/templates/tpl-1/builds/build-1":
			_, _ = w.Write([]byte(`{
				"buildID":"build-1",
				"templateID":"tpl-1",
				"status":"ready",
				"image":"example-image:v1",
				"errorMessage":"",
				"createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-01T00:02:00Z",
				"finishedAt":"2026-01-01T00:02:00Z"
			}`))
		case r.URL.Path == "/api/v1/templates/tpl-1/builds/build-1/logs":
			q := r.URL.Query()
			if q.Get("cursor") != "0" || q.Get("limit") != "10" || q.Get("direction") != "forward" || q.Get("level") != "info" || q.Get("source") != "persistent" {
				t.Fatalf("query = %#v", q)
			}
			_, _ = w.Write([]byte(`{
				"logs":[
					{
						"timestamp":"2026-01-01T00:00:00Z",
						"level":"info",
						"step":"build",
						"message":"building image"
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	history, err := service.ListBuilds(context.Background(), "tpl-1")
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if history.Total != 1 || len(history.Builds) != 1 {
		t.Fatalf("history = %#v", history)
	}

	buildResp, err := service.GetBuild(context.Background(), "tpl-1", "build-1")
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if buildResp.Status != "ready" || buildResp.Image == "" {
		t.Fatalf("build = %#v", buildResp)
	}

	cursor := int64(0)
	limit := 10
	logs, err := service.GetBuildLogs(context.Background(), "tpl-1", "build-1", &build.BuildLogsParams{
		Cursor:    &cursor,
		Limit:     &limit,
		Direction: "forward",
		Level:     "info",
		Source:    "persistent",
	})
	if err != nil {
		t.Fatalf("GetBuildLogs: %v", err)
	}
	if len(logs.Logs) != 1 || logs.Logs[0].Message != "building image" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestDeleteTemplateUsesAuthenticatedTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("X-User-ID"); got != "" {
			t.Fatalf("unexpected user header = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.DeleteTemplate(context.Background(), "tpl-1"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
}

func TestBuildAPIErrorDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"code": 400,
			"message": "validation failed",
			"error": {
				"code": "INVALID_HASH",
				"details": "hash must be sha256"
			},
			"request_id": "req-build-1"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.GetBuildFile(context.Background(), "tpl-1", strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.RequestID != "req-build-1" {
		t.Fatalf("requestID = %q", apiErr.RequestID)
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}

func strPtr(v string) *string {
	return &v
}
