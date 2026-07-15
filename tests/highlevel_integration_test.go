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
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func TestIntegrationHighLevelFacade(t *testing.T) {
	baseURL, apiKey, templateID := integrationConfig(t)
	workspaceRoot := os.Getenv("SANDBOX_TEST_SANDBOX_ROOT")
	if workspaceRoot == "" {
		workspaceRoot = "/root/workspace"
	}
	if templateID == "" {
		t.Skip("SANDBOX_TEST_TEMPLATE_ID is not set")
	}

	client, err := sandbox.NewClient(baseURL, apiKey, core.WithTimeout(180*time.Second))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	waitReady := true
	timeout := int32(1800)
	created, err := client.Create(ctx, templateID, &sandbox.CreateOptions{
		WorkspaceID: "go-facade-sdk-test-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Timeout:     &timeout,
		WaitReady:   &waitReady,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() {
		if err := created.Delete(ctx); err != nil && !isNotFound(err) {
			t.Fatalf("Delete: %v", err)
		}
	}()

	commands, err := created.Commands()
	if err != nil {
		t.Fatalf("Commands: %v", err)
	}
	result, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "echo facade-go"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 || result.Stdout == "" {
		t.Fatalf("result = %#v", result)
	}

	files, err := created.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	path := workspaceRoot + "/go-facade-sdk.txt"
	if _, err := files.Write(ctx, path, []byte("go-facade")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	content, err := files.Read(ctx, path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if content != "go-facade" {
		t.Fatalf("content = %q", content)
	}
	exists, err := files.Exists(ctx, path)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected file to exist")
	}

	pty, err := created.Pty()
	if err != nil {
		t.Fatalf("Pty: %v", err)
	}
	ptyHandle, err := pty.Create(ctx, "sh", &sandbox.PtyCreateOptions{
		Args: []string{"-lc", `printf "ready\n"; IFS= read line; printf "got:%s\n" "$line"`},
		Size: &cmd.PtySize{Cols: 90, Rows: 30},
	})
	if err != nil {
		t.Fatalf("pty.Create: %v", err)
	}
	if err := pty.Resize(ctx, ptyHandle.PID, cmd.PtySize{Cols: 100, Rows: 40}); err != nil {
		t.Fatalf("pty.Resize: %v", err)
	}
	if err := ptyHandle.SendStdin(ctx, "ping\n"); err != nil {
		t.Fatalf("ptyHandle.SendStdin: %v", err)
	}
	ptyResult, err := ptyHandle.Wait(ctx)
	if err != nil {
		t.Fatalf("ptyHandle.Wait: %v", err)
	}
	if !strings.Contains(ptyResult.PTY, "ready") || !strings.Contains(ptyResult.PTY, "got:ping") {
		t.Fatalf("ptyResult = %#v", ptyResult)
	}

	commandHandle, err := commands.Start(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", `IFS= read line; printf "cmd:%s\n" "$line"`},
	})
	if err != nil {
		t.Fatalf("commands.Start: %v", err)
	}
	connectedCommand, err := commands.Connect(ctx, commandHandle.PID)
	if err != nil {
		t.Fatalf("commands.Connect: %v", err)
	}
	if err := connectedCommand.SendStdin(ctx, "pong\n"); err != nil {
		t.Fatalf("connectedCommand.SendStdin: %v", err)
	}
	connectedCommandResult, err := connectedCommand.Wait(ctx)
	if err != nil {
		t.Fatalf("connectedCommand.Wait: %v", err)
	}
	if !strings.Contains(connectedCommandResult.Stdout, "cmd:pong") {
		t.Fatalf("connectedCommandResult = %#v", connectedCommandResult)
	}

	longRunningCommand, err := commands.Start(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "sleep 30"},
	})
	if err != nil {
		t.Fatalf("commands.Start long running: %v", err)
	}
	killedCommand, err := commands.Kill(ctx, longRunningCommand.PID)
	if err != nil || !killedCommand {
		t.Fatalf("commands.Kill = %v, %v", killedCommand, err)
	}
	missingCommand, err := commands.Kill(ctx, longRunningCommand.PID)
	if err != nil || missingCommand {
		t.Fatalf("commands.Kill missing = %v, %v", missingCommand, err)
	}

	ptySource, err := pty.Create(ctx, "sh", &sandbox.PtyCreateOptions{
		Args: []string{"-lc", `IFS= read line; printf "pty:%s\n" "$line"`},
	})
	if err != nil {
		t.Fatalf("pty.Create source: %v", err)
	}
	connectedPty, err := pty.Connect(ctx, ptySource.PID)
	if err != nil {
		t.Fatalf("pty.Connect: %v", err)
	}
	if err := connectedPty.SendStdin(ctx, "echoed\n"); err != nil {
		t.Fatalf("connectedPty.SendStdin: %v", err)
	}
	connectedPtyResult, err := connectedPty.Wait(ctx)
	if err != nil {
		t.Fatalf("connectedPty.Wait: %v", err)
	}
	if !strings.Contains(connectedPtyResult.PTY, "pty:echoed") {
		t.Fatalf("connectedPtyResult = %#v", connectedPtyResult)
	}

	longRunningPty, err := pty.Create(ctx, "sh", &sandbox.PtyCreateOptions{
		Args: []string{"-lc", "sleep 30"},
	})
	if err != nil {
		t.Fatalf("pty.Create long running: %v", err)
	}
	killedPty, err := pty.Kill(ctx, longRunningPty.PID)
	if err != nil || !killedPty {
		t.Fatalf("pty.Kill = %v, %v", killedPty, err)
	}
	missingPty, err := pty.Kill(ctx, longRunningPty.PID)
	if err != nil || missingPty {
		t.Fatalf("pty.Kill missing = %v, %v", missingPty, err)
	}
}
