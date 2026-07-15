package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SeaArt-Infra/sandbox-go/build"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func TestIntegrationBuildPlane(t *testing.T) {
	baseURL, apiKey, image := buildIntegrationConfig(t)

	service, err := build.NewService(baseURL, apiKey)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()

	t.Run("direct build polling", func(t *testing.T) {
		resp, err := service.DirectBuild(ctx, &build.DirectBuildRequest{
			Project:    "sdk-build-integration",
			Image:      "go-direct-build",
			Tag:        fmt.Sprintf("t%d", time.Now().Unix()),
			Dockerfile: "FROM alpine:3.20\nRUN echo direct-build-test >/tmp/direct-build.txt\n",
		})
		if err != nil {
			t.Fatalf("DirectBuild: %v", err)
		}
		if resp.TemplateID == "" || resp.BuildID == "" || resp.ImageFullName == "" {
			t.Fatalf("response = %#v", resp)
		}

		defer func() {
			if err := service.DeleteTemplate(ctx, resp.TemplateID); err != nil && !isBuildNotFound(err) {
				t.Fatalf("DeleteTemplate: %v", err)
			}
		}()

		status := waitForBuildReady(t, ctx, service, resp.TemplateID, resp.BuildID)
		if status.Status != "ready" {
			t.Fatalf("final status = %#v", status)
		}

		buildResp, err := service.GetBuild(ctx, resp.TemplateID, resp.BuildID)
		if err != nil {
			t.Fatalf("GetBuild: %v", err)
		}
		if buildResp.Status != "ready" {
			t.Fatalf("build = %#v", buildResp)
		}

		limit := 10
		logs, err := service.GetBuildLogs(ctx, resp.TemplateID, resp.BuildID, &build.BuildLogsParams{Limit: &limit})
		if err != nil {
			t.Fatalf("GetBuildLogs: %v", err)
		}
		if logs.Logs == nil {
			t.Fatal("logs response is nil")
		}
	})

	t.Run("template lifecycle", func(t *testing.T) {
		name := "go-build-sdk-" + time.Now().UTC().Format("20060102150405")
		created, err := service.CreateTemplate(ctx, &build.TemplateCreateRequest{
			Name: name,
		})
		if err != nil {
			t.Fatalf("CreateTemplate: %v", err)
		}
		if created.TemplateID == "" {
			t.Fatal("template id is empty")
		}

		templateID := created.TemplateID
		buildID := created.BuildID

		if buildID == "" {
			requestedBuildID := fmt.Sprintf("build-%x", time.Now().UnixNano())
			buildResp, err := service.CreateBuild(ctx, templateID, requestedBuildID, &build.BuildRequest{FromImage: image})
			if err != nil {
				t.Fatalf("CreateBuild: %v", err)
			}
			if buildResp == nil {
				t.Fatal("build trigger response is nil")
			}
			buildID = requestedBuildID
		}

		defer func() {
			if err := service.DeleteTemplate(ctx, templateID); err != nil && !isBuildNotFound(err) {
				t.Fatalf("DeleteTemplate: %v", err)
			}
		}()

		listed, err := service.ListTemplates(ctx, &build.ListTemplatesParams{Limit: 20})
		if err != nil {
			t.Fatalf("ListTemplates: %v", err)
		}
		if listed == nil {
			t.Fatal("list response is nil")
		}

		aliased, err := service.GetTemplateByAlias(ctx, name)
		if err != nil {
			t.Fatalf("GetTemplateByAlias: %v", err)
		}
		if aliased.TemplateID != templateID {
			t.Fatalf("alias template id = %q, want %q", aliased.TemplateID, templateID)
		}

		resolved, err := service.ResolveTemplateRef(ctx, templateID)
		if err != nil {
			t.Fatalf("ResolveTemplateRef: %v", err)
		}
		if resolved.TemplateID != templateID {
			t.Fatalf("resolved template id = %q, want %q", resolved.TemplateID, templateID)
		}

		detail, err := service.GetTemplate(ctx, templateID, &build.GetTemplateParams{Limit: 10})
		if err != nil {
			t.Fatalf("GetTemplate: %v", err)
		}
		if detail.TemplateID != templateID {
			t.Fatalf("detail template id = %q, want %q", detail.TemplateID, templateID)
		}
		if len(detail.Builds) == 0 {
			t.Fatal("template detail builds is empty")
		}

		updated, err := service.UpdateTemplate(ctx, templateID, &build.TemplateUpdateRequest{
			Extensions: &build.PublicTemplateExtensions{
				Seacloud: &build.PublicSeacloudTemplateExtensions{
					Envs: map[string]string{"SDK_TEST": "1"},
				},
			},
		})
		if err != nil {
			t.Fatalf("UpdateTemplate: %v", err)
		}
		if len(updated.Names) == 0 {
			t.Fatal("update names is empty")
		}

		fileResp, err := service.GetBuildFile(ctx, templateID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if err != nil {
			t.Fatalf("GetBuildFile: %v", err)
		}
		if fileResp == nil {
			t.Fatal("file response is nil")
		}

		builds, err := service.ListBuilds(ctx, templateID)
		if err != nil {
			t.Fatalf("ListBuilds: %v", err)
		}
		if builds.Total < 0 {
			t.Fatalf("invalid total: %d", builds.Total)
		}

		if buildID == "" {
			t.Skip("build id is empty")
		}

		buildResp, err := service.GetBuild(ctx, templateID, buildID)
		if err != nil {
			t.Fatalf("GetBuild: %v", err)
		}
		if buildResp.BuildID != buildID {
			t.Fatalf("build id = %q, want %q", buildResp.BuildID, buildID)
		}

		status, err := service.GetBuildStatus(ctx, templateID, buildID, &build.BuildStatusParams{Limit: intPtr(10)})
		if err != nil {
			t.Fatalf("GetBuildStatus: %v", err)
		}
		if status.BuildID != buildID {
			t.Fatalf("status build id = %q, want %q", status.BuildID, buildID)
		}

		logs, err := service.GetBuildLogs(ctx, templateID, buildID, &build.BuildLogsParams{Limit: intPtr(10)})
		if err != nil {
			t.Fatalf("GetBuildLogs: %v", err)
		}
		if logs.Logs == nil {
			t.Fatal("logs response is nil")
		}

		rolled, err := service.RollbackTemplate(ctx, templateID, &build.RollbackRequest{BuildID: buildID})
		if err != nil {
			t.Fatalf("RollbackTemplate: %v", err)
		}
		if rolled.TemplateID != templateID {
			t.Fatalf("rollback template id = %q, want %q", rolled.TemplateID, templateID)
		}
	})
}

func buildIntegrationConfig(t *testing.T) (string, string, string) {
	t.Helper()

	if os.Getenv("SANDBOX_RUN_INTEGRATION") != "1" {
		t.Skip("set SANDBOX_RUN_INTEGRATION=1 to run integration tests")
	}

	baseURL := os.Getenv("SANDBOX_TEST_BASE_URL")
	apiKey := os.Getenv("SANDBOX_TEST_API_KEY")
	image := os.Getenv("SANDBOX_TEST_BUILD_IMAGE")
	if image == "" {
		image = "docker.io/library/alpine:3.20"
	}

	if baseURL == "" || apiKey == "" {
		t.Skip("build integration test env is incomplete")
	}

	return baseURL, apiKey, image
}

func isBuildNotFound(err error) bool {
	apiErr, ok := err.(*core.APIError)
	return ok && apiErr.StatusCode == 404
}

func boolPtr(v bool) *bool { return &v }

func waitForBuildReady(
	t *testing.T,
	ctx context.Context,
	service *build.Service,
	templateID, buildID string,
) *build.BuildStatusResponse {
	t.Helper()

	deadline := time.Now().Add(3 * time.Minute)
	limit := 20
	var last *build.BuildStatusResponse

	for time.Now().Before(deadline) {
		status, err := service.GetBuildStatus(ctx, templateID, buildID, &build.BuildStatusParams{Limit: &limit})
		if err != nil {
			t.Fatalf("GetBuildStatus polling: %v", err)
		}
		last = status
		switch status.Status {
		case "ready":
			return status
		case "error":
			t.Fatalf("build failed: %#v", status)
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("build did not complete before deadline, last=%#v", last)
	return nil
}
