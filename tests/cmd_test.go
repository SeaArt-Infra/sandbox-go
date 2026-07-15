package tests

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaArt-Infra/sandbox-go/cmd"
)

func TestCMDListDirSetsConnectHeadersAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/ListDir" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
			t.Fatalf("Connect-Protocol-Version = %q", got)
		}
		if got := r.Header.Get("X-Access-Token"); got != "unit-runtime-auth" {
			t.Fatalf("X-Access-Token = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("sandbox:")) {
			t.Fatalf("Authorization = %q", got)
		}

		var req cmd.ListDirRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Path != "/tmp" {
			t.Fatalf("path = %q", req.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entries":[{"name":"a.txt","type":"FILE_TYPE_FILE","path":"/tmp/a.txt","size":1,"mode":33188,"permissions":"-rw-r--r--","owner":"u","group":"g","modifiedTime":"2026-04-22T00:00:00Z"}]}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.ListDir(context.Background(), &cmd.ListDirRequest{Path: "/tmp"}, &cmd.RequestOptions{Username: "sandbox"})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].Path != "/tmp/a.txt" {
		t.Fatalf("entries = %#v", resp.Entries)
	}
}

func TestCMDDownloadUsesQueryUsernameAndRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("path"); got != "~/hello.txt" {
			t.Fatalf("path query = %q", got)
		}
		if got := r.URL.Query().Get("username"); got != "sandbox" {
			t.Fatalf("username query = %q", got)
		}
		if got := r.Header.Get("Range"); got != "bytes=0-3" {
			t.Fatalf("Range = %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("hell"))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Download(context.Background(), &cmd.DownloadRequest{Path: "~/hello.txt"}, &cmd.RequestOptions{
		Username: "sandbox",
		Range:    "bytes=0-3",
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "hell" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestCMDPortsAcceptsStatusObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ports" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Ports(context.Background())
	if err != nil {
		t.Fatalf("Ports: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("ports = %#v", resp)
	}
}

func TestCMDEnvsConfigureAndPorts(t *testing.T) {
	calls := make([]struct {
		path   string
		method string
		body   map[string]any
	}, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		calls = append(calls, struct {
			path   string
			method string
			body   map[string]any
		}{path: r.URL.Path, method: r.Method, body: body})
		switch r.URL.Path {
		case "/envs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"NODE_ENV":"production"}`))
		case "/configure":
			w.WriteHeader(http.StatusNoContent)
		case "/ports":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"port":3000,"protocol":"tcp","address":"127.0.0.1"}]`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	envs, err := service.Envs(context.Background())
	if err != nil {
		t.Fatalf("Envs: %v", err)
	}
	if envs["NODE_ENV"] != "production" {
		t.Fatalf("envs = %#v", envs)
	}
	if err := service.Configure(context.Background(), &cmd.ConfigureRequest{Envs: map[string]string{"A": "1"}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	ports, err := service.Ports(context.Background())
	if err != nil {
		t.Fatalf("Ports: %v", err)
	}
	if len(ports) != 1 || ports[0].Port != 3000 {
		t.Fatalf("ports = %#v", ports)
	}
	if calls[1].path != "/configure" || calls[1].body["envs"].(map[string]any)["A"] != "1" {
		t.Fatalf("configure call = %#v", calls[1])
	}
}

func TestCMDWatcherAndFileHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/filesystem.Filesystem/CreateWatcher":
			var req cmd.CreateWatcherRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create watcher: %v", err)
			}
			if req.Path != "/tmp" || req.Recursive == nil || !*req.Recursive {
				t.Fatalf("create watcher req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"watcherId":"watch-1"}`))
		case "/filesystem.Filesystem/GetWatcherEvents":
			var req cmd.GetWatcherEventsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode watcher events: %v", err)
			}
			if req.WatcherID != "watch-1" || req.Limit == nil || *req.Limit != 10 {
				t.Fatalf("watcher events req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"events":[{"name":"a.txt","type":"EVENT_TYPE_WRITE"}]}`))
		case "/filesystem.Filesystem/RemoveWatcher":
			var req cmd.RemoveWatcherRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode remove watcher: %v", err)
			}
			if req.WatcherID != "watch-1" {
				t.Fatalf("remove watcher req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case "/files":
			if got := r.URL.Query().Get("path"); got != "/tmp" {
				t.Fatalf("query path = %q", got)
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
				t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Fatalf("ParseMultipartForm: %v", err)
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("FormFile: %v", err)
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if header.Filename != "hello.txt" || string(body) != "hello" {
				t.Fatalf("multipart = %q %q", header.Filename, string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"path":"/tmp/hello.txt","name":"hello.txt","type":"file"}]`))
		case "/files/compose":
			var req cmd.ComposeFilesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode compose: %v", err)
			}
			if strings.Join(req.SourcePaths, "|") != "/tmp/a.txt|/tmp/b.txt" || req.Destination != "/tmp/out.txt" {
				t.Fatalf("compose req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"path":"/tmp/out.txt","name":"out.txt","type":"file"}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	recursive := true
	watcher, err := service.CreateWatcher(context.Background(), &cmd.CreateWatcherRequest{Path: "/tmp", Recursive: &recursive}, nil)
	if err != nil {
		t.Fatalf("CreateWatcher: %v", err)
	}
	limit := 10
	events, err := service.GetWatcherEvents(context.Background(), &cmd.GetWatcherEventsRequest{WatcherID: watcher.WatcherID, Limit: &limit}, nil)
	if err != nil {
		t.Fatalf("GetWatcherEvents: %v", err)
	}
	if len(events.Events) != 1 || events.Events[0].Name != "a.txt" {
		t.Fatalf("events = %#v", events.Events)
	}
	if err := service.RemoveWatcher(context.Background(), &cmd.RemoveWatcherRequest{WatcherID: watcher.WatcherID}, nil); err != nil {
		t.Fatalf("RemoveWatcher: %v", err)
	}
	uploaded, err := service.UploadMultipart(context.Background(), &cmd.UploadMultipartRequest{
		Path: "/tmp",
		Parts: []cmd.MultipartFile{{
			FileName:    "hello.txt",
			ContentType: "text/plain",
			Data:        []byte("hello"),
		}},
	}, nil)
	if err != nil {
		t.Fatalf("UploadMultipart: %v", err)
	}
	if len(uploaded) != 1 || uploaded[0].Path != "/tmp/hello.txt" {
		t.Fatalf("uploaded = %#v", uploaded)
	}
	composed, err := service.ComposeFiles(context.Background(), &cmd.ComposeFilesRequest{
		SourcePaths: []string{"/tmp/a.txt", "/tmp/b.txt"},
		Destination: "/tmp/out.txt",
	}, nil)
	if err != nil {
		t.Fatalf("ComposeFiles: %v", err)
	}
	if composed.Path != "/tmp/out.txt" {
		t.Fatalf("composed = %#v", composed)
	}
	if _, err := service.GetWatcherEvents(context.Background(), &cmd.GetWatcherEventsRequest{WatcherID: " "}, nil); !errors.Is(err, cmd.ErrWatcherIDEmpty) {
		t.Fatalf("GetWatcherEvents error = %v", err)
	}
	if err := service.RemoveWatcher(context.Background(), &cmd.RemoveWatcherRequest{WatcherID: " "}, nil); !errors.Is(err, cmd.ErrWatcherIDEmpty) {
		t.Fatalf("RemoveWatcher error = %v", err)
	}
	if _, err := service.UploadMultipart(context.Background(), &cmd.UploadMultipartRequest{}, nil); !errors.Is(err, cmd.ErrMultipartFilesEmpty) {
		t.Fatalf("UploadMultipart error = %v", err)
	}
}

func TestCMDFilesContentUploadBytesUploadJSONAndEdit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/content":
			if r.URL.Query().Get("path") != "/tmp/a.txt" || r.URL.Query().Get("max_tokens") != "32" {
				t.Fatalf("query = %#v", r.URL.Query())
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"text","content":"hello","truncated":false}`))
		case "/files":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if r.URL.Query().Get("path") != "" {
				if r.URL.Query().Get("path") != "/tmp/a.txt" {
					t.Fatalf("query path = %q", r.URL.Query().Get("path"))
				}
				if r.Header.Get("Content-Encoding") != "gzip" {
					t.Fatalf("Content-Encoding = %q", r.Header.Get("Content-Encoding"))
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll: %v", err)
				}
				if len(body) < 2 || body[0] != 0x1f || body[1] != 0x8b {
					t.Fatalf("gzip header = %v", body[:min(2, len(body))])
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"path":"/tmp/a.txt","name":"a.txt","type":"file"}]`))
				return
			}
			var req cmd.WriteFileEntry
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode upload json: %v", err)
			}
			if req.Path != "/tmp/b.txt" || req.Content != "hello" {
				t.Fatalf("upload json req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"path":"/tmp/b.txt","name":"b.txt","type":"file"}]`))
		case "/filesystem.Filesystem/Edit":
			var req cmd.FsEditRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode edit: %v", err)
			}
			if req.Path != "/tmp/a.txt" || req.OldText != "a" || req.NewText != "b" {
				t.Fatalf("edit req = %#v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"ok"}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	maxTokens := 32
	content, err := service.FilesContent(context.Background(), &cmd.FilesContentRequest{Path: "/tmp/a.txt", MaxTokens: &maxTokens}, nil)
	if err != nil || content.Content != "hello" {
		t.Fatalf("FilesContent = %#v, %v", content, err)
	}
	uploaded, err := service.UploadBytes(context.Background(), &cmd.UploadBytesRequest{
		Path:         "/tmp/a.txt",
		Data:         []byte("hello"),
		GzipCompress: true,
	}, nil)
	if err != nil || len(uploaded) != 1 || uploaded[0].Path != "/tmp/a.txt" {
		t.Fatalf("UploadBytes = %#v, %v", uploaded, err)
	}
	uploadedJSON, err := service.UploadJSON(context.Background(), &cmd.WriteFileEntry{Path: "/tmp/b.txt", Content: "hello"}, nil)
	if err != nil || len(uploadedJSON) != 1 || uploadedJSON[0].Path != "/tmp/b.txt" {
		t.Fatalf("UploadJSON = %#v, %v", uploadedJSON, err)
	}
	edited, err := service.Edit(context.Background(), &cmd.FsEditRequest{Path: "/tmp/a.txt", OldText: "a", NewText: "b"}, nil)
	if err != nil || edited.Message != "ok" {
		t.Fatalf("Edit = %#v, %v", edited, err)
	}
}

func TestCMDInvalidProcessAndPathInputs(t *testing.T) {
	service, err := cmd.NewService("https://sandbox-gateway.cloud.seaart.ai", "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := service.CreateWatcher(context.Background(), &cmd.CreateWatcherRequest{Path: " "}, nil); !errors.Is(err, cmd.ErrPathEmpty) {
		t.Fatalf("CreateWatcher error = %v", err)
	}
	if _, err := service.FilesContent(context.Background(), &cmd.FilesContentRequest{Path: " "}, nil); !errors.Is(err, cmd.ErrPathEmpty) {
		t.Fatalf("FilesContent error = %v", err)
	}
	if _, err := service.UploadJSON(context.Background(), &cmd.WriteFileEntry{Path: " "}, nil); !errors.Is(err, cmd.ErrPathEmpty) {
		t.Fatalf("UploadJSON error = %v", err)
	}
	if _, err := service.Edit(context.Background(), &cmd.FsEditRequest{Path: " ", OldText: "a", NewText: "b"}, nil); !errors.Is(err, cmd.ErrPathEmpty) {
		t.Fatalf("Edit error = %v", err)
	}
	if _, err := service.StreamInput(context.Background(), nil, nil); !errors.Is(err, cmd.ErrStreamInputFramesZero) {
		t.Fatalf("StreamInput error = %v", err)
	}
	if err := service.SendInput(context.Background(), &cmd.SendInputRequest{Process: cmd.ProcessSelector{}, Input: cmd.ProcessInput{}}, nil); !errors.Is(err, cmd.ErrProcessSelectorEmpty) {
		t.Fatalf("SendInput selector error = %v", err)
	}
	if err := service.SendInput(context.Background(), &cmd.SendInputRequest{Process: cmd.ProcessSelector{PID: 1, Tag: "x"}, Input: cmd.ProcessInput{Stdin: "x"}}, nil); !errors.Is(err, cmd.ErrProcessSelectorAmbig) {
		t.Fatalf("SendInput ambiguous error = %v", err)
	}
	if err := service.SendSignal(context.Background(), &cmd.SendSignalRequest{Process: cmd.ProcessSelector{}}, nil); !errors.Is(err, cmd.ErrProcessSelectorEmpty) {
		t.Fatalf("SendSignal error = %v", err)
	}
	if err := service.CloseStdin(context.Background(), &cmd.CloseStdinRequest{Process: cmd.ProcessSelector{}}, nil); !errors.Is(err, cmd.ErrProcessSelectorEmpty) {
		t.Fatalf("CloseStdin error = %v", err)
	}
	if _, err := service.GetResult(context.Background(), &cmd.GetResultRequest{CmdID: " "}, nil); !errors.Is(err, cmd.ErrCmdIDEmpty) {
		t.Fatalf("GetResult error = %v", err)
	}
}

func TestCMDPreservesBaseURLPathPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandbox/sb-1/run" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stdout":"ok","stderr":"","exit_code":0,"duration_ms":1}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL+"/sandbox/sb-1", "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Run(context.Background(), &cmd.AgentRunRequest{Cmd: "echo"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Stdout != "ok" {
		t.Fatalf("stdout = %q", resp.Stdout)
	}
}

func TestCMDProxyPassesThroughNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/8080/health" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream failed"))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Proxy(context.Background(), &cmd.ProxyRequest{
		Method: http.MethodGet,
		Port:   8080,
		Path:   "/health",
	})
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "upstream failed" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestCMDProcessStreamParsesConnectFrames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
			t.Fatalf("Content-Type = %q", got)
		}
		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameJSON(t, map[string]any{
			"event": map[string]any{
				"start": map[string]any{"pid": 1234, "cmdId": "cmd-1"},
			},
		}))
		_, _ = w.Write(connectFrameJSON(t, map[string]any{
			"event": map[string]any{
				"data": map[string]any{"stdout": base64.StdEncoding.EncodeToString([]byte("hello\n"))},
			},
		}))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	stream, err := service.Start(context.Background(), &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{Cmd: "echo", Args: []string{"hello"}},
	}, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if first.Event.Start == nil || first.Event.Start.CmdID != "cmd-1" {
		t.Fatalf("start event = %#v", first.Event.Start)
	}

	second, err := stream.Next()
	if err != nil {
		t.Fatalf("second Next: %v", err)
	}
	if second.Event.Data == nil || second.Event.Data.Stdout == "" {
		t.Fatalf("data event = %#v", second.Event.Data)
	}
}

func TestCMDWatchDirSkipsKeepaliveAndStopsOnEndFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/WatchDir" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
			t.Fatalf("Content-Type = %q", got)
		}
		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameWithFlagsJSON(t, 0, nil))
		_, _ = w.Write(connectFrameJSON(t, map[string]any{
			"filesystem": map[string]any{"type": "EVENT_TYPE_WRITE", "name": "a.txt"},
		}))
		_, _ = w.Write(connectFrameWithFlagsJSON(t, 0x02, nil))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	recursive := true
	stream, err := service.WatchDir(context.Background(), &cmd.WatchDirRequest{
		Path:      "/tmp",
		Recursive: &recursive,
	}, nil)
	if err != nil {
		t.Fatalf("WatchDir: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if first.Filesystem == nil || first.Filesystem.Name != "a.txt" || first.Filesystem.Type != "EVENT_TYPE_WRITE" {
		t.Fatalf("filesystem event = %#v", first.Filesystem)
	}

	second, err := stream.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("second Next err = %v", err)
	}
	if second != nil {
		t.Fatalf("second = %#v", second)
	}
}

func TestCMDStreamInputEncodesConnectFrames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/StreamInput" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
			t.Fatalf("Content-Type = %q", got)
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		frames := decodeFrames(t, data)
		if len(frames) != 2 {
			t.Fatalf("frames len = %d", len(frames))
		}
		if !strings.Contains(string(frames[0].Payload), `"pid":42`) {
			t.Fatalf("start payload = %s", string(frames[0].Payload))
		}
		if !strings.Contains(string(frames[1].Payload), `"stdin":"aGVsbG8="`) {
			t.Fatalf("data payload = %s", string(frames[1].Payload))
		}

		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameJSON(t, map[string]any{}))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	frame, err := service.StreamInput(context.Background(), []cmd.StreamInputFrame{
		{Start: &cmd.StreamInputStart{Process: cmd.ProcessSelector{PID: 42}}},
		{Data: &cmd.StreamInputData{Input: cmd.ProcessInput{Stdin: base64.StdEncoding.EncodeToString([]byte("hello"))}}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamInput: %v", err)
	}
	if frame == nil {
		t.Fatal("frame is nil")
	}
}

func TestCMDStreamInputReturnsRawEndFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/StreamInput" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameWithFlagsJSON(t, 0x02, nil))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	frame, err := service.StreamInput(context.Background(), []cmd.StreamInputFrame{
		{Keepalive: &struct{}{}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamInput: %v", err)
	}
	if frame == nil || frame.Flags != 0x02 || len(frame.Payload) != 0 {
		t.Fatalf("frame = %#v", frame)
	}
}

func TestCMDUpdateRequiresPTY(t *testing.T) {
	service, err := cmd.NewService("https://sandbox-gateway.cloud.seaart.ai", "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = service.Update(context.Background(), &cmd.UpdateRequest{
		Process: cmd.ProcessSelector{PID: 42},
	}, nil)
	if !errors.Is(err, cmd.ErrPTYRequired) {
		t.Fatalf("Update error = %v", err)
	}
}

func connectFrameJSON(t *testing.T, payload any) []byte {
	return connectFrameWithFlagsJSON(t, 0, payload)
}

func connectFrameWithFlagsJSON(t *testing.T, flags byte, payload any) []byte {
	t.Helper()

	var data []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		data = encoded
	}

	out := make([]byte, 5+len(data))
	out[0] = flags
	binary.BigEndian.PutUint32(out[1:5], uint32(len(data)))
	copy(out[5:], data)
	return out
}

func decodeFrames(t *testing.T, data []byte) []cmd.ConnectFrame {
	t.Helper()

	var frames []cmd.ConnectFrame
	for len(data) > 0 {
		if len(data) < 5 {
			t.Fatalf("short frame header: %d", len(data))
		}
		size := binary.BigEndian.Uint32(data[1:5])
		if len(data) < int(5+size) {
			t.Fatalf("short frame payload: %d < %d", len(data), 5+size)
		}
		payload := append([]byte(nil), data[5:5+size]...)
		frames = append(frames, cmd.ConnectFrame{
			Flags:   data[0],
			Payload: payload,
		})
		data = data[5+size:]
	}
	return frames
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
