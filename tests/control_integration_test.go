package tests

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	sandbox "github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func TestIntegrationControlPlane(t *testing.T) {
	baseURL, apiKey, templateID := integrationConfig(t)

	service, err := control.NewService(baseURL, apiKey, core.WithTimeout(180*time.Second))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()

	t.Run("list sandboxes", func(t *testing.T) {
		resp, err := service.ListSandboxes(ctx, &control.ListSandboxesParams{Limit: 10})
		if err != nil {
			t.Fatalf("ListSandboxes: %v", err)
		}
		if resp == nil {
			t.Fatal("list response is nil")
		}
	})

	t.Run("pool status", func(t *testing.T) {
		resp, err := service.GetPoolStatus(ctx)
		if err != nil {
			if isNotFound(err) {
				t.Skip("admin pool status is not exposed by this gateway")
			}
			t.Fatalf("GetPoolStatus: %v", err)
		}
		if resp.Total < 0 {
			t.Fatalf("invalid total: %d", resp.Total)
		}
	})

	t.Run("rolling status", func(t *testing.T) {
		resp, err := service.GetRollingUpdateStatus(ctx)
		if err != nil {
			if isNotFound(err) {
				t.Skip("admin rolling status is not exposed by this gateway")
			}
			t.Fatalf("GetRollingUpdateStatus: %v", err)
		}
		if resp.Phase == "" {
			t.Fatal("rolling phase is empty")
		}
	})

	t.Run("sandbox lifecycle", func(t *testing.T) {
		if templateID == "" {
			t.Skip("SANDBOX_TEST_TEMPLATE_ID is not set")
		}

		workspaceID := "go-sdk-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		timeout := int32(1800)
		waitReady := true

		created, err := service.CreateSandbox(ctx, &control.NewSandboxRequest{
			TemplateID:  templateID,
			WorkspaceID: workspaceID,
			Timeout:     &timeout,
			WaitReady:   &waitReady,
		})
		if err != nil {
			t.Fatalf("CreateSandbox: %v", err)
		}

		sandboxID := created.SandboxID
		if sandboxID == "" {
			t.Fatal("created sandbox id is empty")
		}
		defer func() {
			if err := service.DeleteSandbox(ctx, sandboxID); err != nil && !isNotFound(err) {
				t.Fatalf("DeleteSandbox: %v", err)
			}
		}()

		detail, err := service.GetSandbox(ctx, sandboxID)
		if err != nil {
			t.Fatalf("GetSandbox: %v", err)
		}
		if detail.SandboxID != sandboxID {
			t.Fatalf("detail sandbox id = %q, want %q", detail.SandboxID, sandboxID)
		}

		hb, err := service.SendHeartbeat(ctx, sandboxID, &control.HeartbeatRequest{
			Status: "healthy",
		})
		if err != nil {
			t.Fatalf("SendHeartbeat: %v", err)
		}
		if !hb.Received {
			t.Fatal("heartbeat not received")
		}

		if err := service.SetSandboxTimeout(ctx, sandboxID, &control.TimeoutRequest{Timeout: 1200}); err != nil {
			t.Fatalf("SetSandboxTimeout: %v", err)
		}

		refresh := int32(60)
		if err := service.RefreshSandbox(ctx, sandboxID, &control.RefreshSandboxRequest{Duration: &refresh}); err != nil {
			t.Fatalf("RefreshSandbox: %v", err)
		}
		if err := service.RefreshSandbox(ctx, sandboxID, nil); err != nil {
			t.Fatalf("RefreshSandbox nil: %v", err)
		}

		logs, err := service.GetSandboxLogs(ctx, sandboxID, &control.SandboxLogsParams{Limit: intPtr(10)})
		if err != nil {
			t.Fatalf("GetSandboxLogs: %v", err)
		}
		if logs.Logs == nil {
			t.Fatal("logs response is nil")
		}

		if err := service.PauseSandbox(ctx, sandboxID); err != nil {
			t.Fatalf("PauseSandbox: %v", err)
		}

		connected, err := service.ConnectSandbox(ctx, sandboxID, &control.ConnectSandboxRequest{Timeout: 1200})
		if err != nil {
			t.Fatalf("ConnectSandbox: %v", err)
		}
		if connected.StatusCode != 200 && connected.StatusCode != 201 {
			t.Fatalf("connect status = %d", connected.StatusCode)
		}
		if connected.Sandbox.EnvdURL != nil && strings.TrimSpace(*connected.Sandbox.EnvdURL) != "" {
			client, err := sandbox.NewClient(baseURL, apiKey, core.WithTimeout(180*time.Second))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			runtime, err := client.RuntimeFromSandbox(connected.Sandbox)
			if err != nil {
				t.Fatalf("RuntimeFromSandbox: %v", err)
			}
			run, err := runtime.Run(ctx, &cmd.AgentRunRequest{
				Cmd:  "sh",
				Args: []string{"-lc", "echo resumed-go"},
			}, nil)
			if err != nil {
				t.Fatalf("Run after resume: %v", err)
			}
			if run.ExitCode != 0 {
				t.Fatalf("run exit code = %d", run.ExitCode)
			}
			if !strings.Contains(run.Stdout, "resumed-go") {
				t.Fatalf("stdout = %q", run.Stdout)
			}
		}
	})
}

func isNotFound(err error) bool {
	apiErr, ok := err.(*core.APIError)
	return ok && apiErr.StatusCode == 404
}

func integrationConfig(t *testing.T) (string, string, string) {
	t.Helper()

	if os.Getenv("SANDBOX_RUN_INTEGRATION") != "1" {
		t.Skip("set SANDBOX_RUN_INTEGRATION=1 to run integration tests")
	}

	baseURL := os.Getenv("SANDBOX_TEST_BASE_URL")
	apiKey := os.Getenv("SANDBOX_TEST_API_KEY")
	templateID := os.Getenv("SANDBOX_TEST_TEMPLATE_ID")

	if baseURL == "" || apiKey == "" {
		t.Skip("integration test env is incomplete")
	}

	return baseURL, apiKey, templateID
}

func intPtr(v int) *int {
	return &v
}
