package tests

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
)

func TestIntegrationCMD(t *testing.T) {
	baseURL, apiKey, templateID := integrationConfig(t)
	workspaceRoot := os.Getenv("SANDBOX_TEST_SANDBOX_ROOT")
	if workspaceRoot == "" {
		workspaceRoot = "/root/workspace"
	}

	if templateID == "" {
		t.Skip("SANDBOX_TEST_TEMPLATE_ID is not set")
	}

	client, err := sandbox.NewClient(baseURL, apiKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	workspaceID := "go-cmd-sdk-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	timeout := int32(1800)
	waitReady := true

	created, err := client.CreateSandbox(ctx, &control.NewSandboxRequest{
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
		if err := client.DeleteSandbox(ctx, sandboxID); err != nil && !isNotFound(err) {
			t.Fatalf("DeleteSandbox: %v", err)
		}
	}()

	if created.EnvdURL == nil || strings.TrimSpace(*created.EnvdURL) == "" {
		t.Skip("sandbox did not return envdUrl")
	}

	accessToken := ""
	if created.EnvdAccessToken != nil {
		accessToken = *created.EnvdAccessToken
	}

	command, err := client.Runtime(*created.EnvdURL, accessToken)
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}

	t.Run("rest files", func(t *testing.T) {
		path := strings.TrimRight(workspaceRoot, "/") + "/go-cmd-sdk.txt"
		resp, err := command.UploadBytes(ctx, &cmd.UploadBytesRequest{
			Path: path,
			Data: []byte("go-cmd"),
		}, nil)
		if err != nil {
			t.Fatalf("UploadBytes: %v", err)
		}
		if len(resp) == 0 {
			t.Fatal("upload response is empty")
		}

		download, err := command.Download(ctx, &cmd.DownloadRequest{Path: path}, nil)
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		defer download.Body.Close()

		body, err := io.ReadAll(download.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(body) != "go-cmd" {
			t.Fatalf("body = %q", string(body))
		}

		content, err := command.FilesContent(ctx, &cmd.FilesContentRequest{Path: path}, nil)
		if err != nil {
			t.Fatalf("FilesContent: %v", err)
		}
		if content.Type != "text" {
			t.Fatalf("content type = %q", content.Type)
		}
		if content.Content != "go-cmd" {
			t.Fatalf("content = %q", content.Content)
		}

		baseDir := strings.TrimRight(workspaceRoot, "/") + "/go-cmd-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if _, err := command.MakeDir(ctx, &cmd.MakeDirRequest{Path: baseDir}, nil); err != nil {
			t.Fatalf("MakeDir: %v", err)
		}
		jsonPath := baseDir + "/json.txt"
		gzipPath := baseDir + "/gzip.txt"
		movedPath := baseDir + "/moved.txt"
		batchAPath := baseDir + "/batch-a.txt"
		batchBPath := baseDir + "/batch-b.txt"
		composedPath := baseDir + "/joined.txt"

		if _, err := command.UploadJSON(ctx, &cmd.WriteFileEntry{Path: jsonPath, Content: "alpha"}, nil); err != nil {
			t.Fatalf("UploadJSON: %v", err)
		}
		if _, err := command.Edit(ctx, &cmd.FsEditRequest{Path: jsonPath, OldText: "alpha", NewText: "beta"}, nil); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if _, err := command.UploadBytes(ctx, &cmd.UploadBytesRequest{
			Path:         gzipPath,
			Data:         []byte("gzip-go"),
			GzipCompress: true,
		}, nil); err != nil {
			t.Fatalf("UploadBytes gzip: %v", err)
		}
		if _, err := command.Move(ctx, &cmd.MoveRequest{Source: jsonPath, Destination: movedPath}, nil); err != nil {
			t.Fatalf("Move: %v", err)
		}
		batch, err := command.WriteBatch(ctx, &cmd.WriteFilesRequest{
			Files: []cmd.WriteFileEntry{
				{Path: batchAPath, Content: "A"},
				{Path: batchBPath, Content: "B"},
			},
		}, nil)
		if err != nil {
			t.Fatalf("WriteBatch: %v", err)
		}
		if len(batch.Files) != 2 {
			t.Fatalf("batch = %#v", batch.Files)
		}
		gzipBody, err := waitForDownloadedText(ctx, command, gzipPath)
		if err != nil {
			t.Fatalf("Download gzip: %v", err)
		}
		if gzipBody != "gzip-go" {
			t.Fatalf("gzip body = %q", gzipBody)
		}
		if _, err := command.ComposeFiles(ctx, &cmd.ComposeFilesRequest{
			SourcePaths: []string{movedPath, gzipPath},
			Destination: composedPath,
		}, nil); err != nil {
			t.Fatalf("ComposeFiles: %v", err)
		}
		composedBody, err := waitForDownloadedText(ctx, command, composedPath)
		if err != nil {
			t.Fatalf("Download composed: %v", err)
		}
		if !strings.Contains(composedBody, "beta") || !strings.Contains(composedBody, "gzip-go") {
			t.Fatalf("composed body = %q", composedBody)
		}
		listDepth := 1
		listing, err := command.ListDir(ctx, &cmd.ListDirRequest{Path: baseDir, Depth: &listDepth}, nil)
		if err != nil {
			t.Fatalf("ListDir nested: %v", err)
		}
		foundComposed := false
		for _, entry := range listing.Entries {
			if entry.Path == composedPath {
				foundComposed = true
			}
			if entry.Path == gzipPath || entry.Path == movedPath {
				t.Fatalf("source should be deleted after compose, listing = %#v", listing.Entries)
			}
		}
		if !foundComposed {
			t.Fatalf("listing = %#v", listing.Entries)
		}
		if err := command.Remove(ctx, &cmd.RemoveRequest{Path: composedPath}, nil); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	})

	t.Run("list dir", func(t *testing.T) {
		depth := 1
		resp, err := command.ListDir(ctx, &cmd.ListDirRequest{
			Path:  workspaceRoot,
			Depth: &depth,
		}, nil)
		if err != nil {
			t.Fatalf("ListDir: %v", err)
		}
		if resp.Entries == nil {
			t.Fatal("entries is nil")
		}
	})

	t.Run("watcher events", func(t *testing.T) {
		watchRoot := "/tmp"
		fileName := "go-watch-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".txt"
		watcher, err := command.CreateWatcher(ctx, &cmd.CreateWatcherRequest{
			Path: watchRoot,
		}, nil)
		if err != nil {
			if isWatcherUnsupported(err) {
				t.Skip("watcher is not supported by this sandbox filesystem layout")
			}
			t.Fatalf("CreateWatcher: %v", err)
		}
		defer func() {
			if err := command.RemoveWatcher(ctx, &cmd.RemoveWatcherRequest{WatcherID: watcher.WatcherID}, nil); err != nil {
				t.Fatalf("RemoveWatcher: %v", err)
			}
		}()

		if _, err := command.UploadBytes(ctx, &cmd.UploadBytesRequest{
			Path: watchRoot + "/" + fileName,
			Data: []byte("watch-go"),
		}, nil); err != nil {
			t.Fatalf("UploadBytes: %v", err)
		}

		events, err := waitForWatcherEvent(ctx, command, watcher.WatcherID, fileName)
		if err != nil {
			t.Fatalf("waitForWatcherEvent: %v", err)
		}
		found := false
		for _, event := range events {
			if event.Name == fileName {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("events = %#v", events)
		}
	})

	t.Run("process stream", func(t *testing.T) {
		stream, err := command.Start(ctx, &cmd.ProcessStartRequest{
			Process: &cmd.ProcessConfig{Cmd: "cat"},
			Tag:     "go-cmd-test",
		}, nil)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer func() {
			_ = stream.Close()
		}()

		startFrame, err := stream.Next()
		if err != nil {
			t.Fatalf("stream.Next start: %v", err)
		}
		if startFrame.Event.Start == nil {
			t.Fatalf("expected start event, got %#v", startFrame.Event)
		}

		pid := startFrame.Event.Start.PID
		cmdID := startFrame.Event.Start.CmdID
		if pid == 0 {
			t.Fatal("start event pid is empty")
		}
		processList, err := command.ListProcesses(ctx, nil)
		if err != nil {
			t.Fatalf("ListProcesses: %v", err)
		}
		foundPID := false
		for _, process := range processList.Processes {
			if process.PID == pid {
				foundPID = true
				break
			}
		}
		if !foundPID {
			t.Fatalf("processes = %#v", processList.Processes)
		}

		if err := command.SendInput(ctx, &cmd.SendInputRequest{
			Process: cmd.ProcessSelector{PID: pid},
			Input: cmd.ProcessInput{
				Stdin: base64.StdEncoding.EncodeToString([]byte("ping\n")),
			},
		}, nil); err != nil {
			t.Fatalf("SendInput: %v", err)
		}

		if err := command.CloseStdin(ctx, &cmd.CloseStdinRequest{
			Process: cmd.ProcessSelector{PID: pid},
		}, nil); err != nil {
			t.Fatalf("CloseStdin: %v", err)
		}

		sawOutput := false
		sawEnd := false
		for i := 0; i < 10; i++ {
			frame, err := stream.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("stream.Next: %v", err)
			}
			if frame.Event.Data != nil && frame.Event.Data.Stdout != "" {
				data, err := base64.StdEncoding.DecodeString(frame.Event.Data.Stdout)
				if err != nil {
					t.Fatalf("DecodeString: %v", err)
				}
				if strings.Contains(string(data), "ping") {
					sawOutput = true
				}
			}
			if frame.Event.End != nil {
				sawEnd = true
				break
			}
		}

		if !sawOutput {
			t.Fatal("did not observe expected stdout")
		}
		if !sawEnd {
			t.Fatal("did not observe process end event")
		}
		result, err := command.GetResult(ctx, &cmd.GetResultRequest{CmdID: cmdID}, nil)
		if err != nil {
			t.Fatalf("GetResult: %v", err)
		}
		if result.ExitCode != 0 || !strings.Contains(result.Stdout, "ping") {
			t.Fatalf("result = %#v", result)
		}
	})
}

func waitForWatcherEvent(ctx context.Context, command *sandbox.Runtime, watcherID string, fileName string) ([]cmd.FilesystemEvent, error) {
	for i := 0; i < 12; i++ {
		limit := 20
		resp, err := command.GetWatcherEvents(ctx, &cmd.GetWatcherEventsRequest{
			WatcherID: watcherID,
			Limit:     &limit,
		}, nil)
		if err != nil {
			return nil, err
		}
		for _, event := range resp.Events {
			if event.Name == fileName {
				return resp.Events, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, nil
}

func waitForDownloadedText(ctx context.Context, command *sandbox.Runtime, path string) (string, error) {
	for i := 0; i < 8; i++ {
		resp, err := command.Download(ctx, &cmd.DownloadRequest{Path: path}, nil)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				return "", readErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			return string(body), nil
		}
		if !isNotFound(err) || i == 7 {
			return "", err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return "", nil
}

func isWatcherUnsupported(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "network filesystem") || strings.Contains(message, "outside allowed directory")
}
