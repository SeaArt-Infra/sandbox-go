package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Service) Metrics(ctx context.Context) (*MetricsResponse, error) {
	var resp MetricsResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/metrics", nil, nil, &resp, "", "", nil, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) Envs(ctx context.Context) (map[string]string, error) {
	var resp map[string]string
	if _, err := c.doJSON(ctx, http.MethodGet, "/envs", nil, nil, &resp, "", "", nil, http.StatusOK); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) Configure(ctx context.Context, req *ConfigureRequest) error {
	if req == nil {
		req = &ConfigureRequest{}
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/configure", nil, req, nil, "", "", nil, http.StatusNoContent)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) Ports(ctx context.Context) ([]PortEntry, error) {
	var raw json.RawMessage
	if _, err := c.doJSON(ctx, http.MethodGet, "/ports", nil, nil, &raw, "", "", nil, http.StatusOK); err != nil {
		return nil, err
	}
	// Older runtime images can return {"status":"ok"} when no port inventory is available.
	var resp []PortEntry
	if json.Unmarshal(raw, &resp) == nil {
		return resp, nil
	}
	var wrapped struct {
		Ports []PortEntry `json:"ports"`
		Data  []PortEntry `json:"data"`
	}
	if json.Unmarshal(raw, &wrapped) == nil {
		if wrapped.Ports != nil {
			return wrapped.Ports, nil
		}
		if wrapped.Data != nil {
			return wrapped.Data, nil
		}
	}
	return []PortEntry{}, nil
}

func (c *Service) Proxy(ctx context.Context, req *ProxyRequest) (*http.Response, error) {
	if req == nil || req.Port <= 0 {
		return nil, ErrPortInvalid
	}

	method := req.Method
	if strings.TrimSpace(method) == "" {
		method = http.MethodGet
	}
	path := "/proxy/" + strconv.Itoa(req.Port) + "/"
	if trimmed := strings.TrimPrefix(req.Path, "/"); trimmed != "" {
		path += trimmed
	}

	httpReq, err := c.newRequest(ctx, method, path, nil, req.Body, "", "*/*", &RequestOptions{Headers: req.Headers})
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(httpReq)
}

func (c *Service) Download(ctx context.Context, req *DownloadRequest, opts *RequestOptions) (*http.Response, error) {
	if req == nil {
		return nil, ErrPathEmpty
	}
	query, err := fileQuery(req.Path, opts)
	if err != nil {
		return nil, err
	}
	requestOpts := cloneOptionsWithHeaders(opts, nil)
	if requestOpts != nil && strings.TrimSpace(requestOpts.Range) != "" {
		if requestOpts.Headers == nil {
			requestOpts.Headers = make(http.Header)
		}
		requestOpts.Headers.Set("Range", requestOpts.Range)
	}
	return c.do(ctx, http.MethodGet, "/files", query, nil, "", "*/*", requestOpts, http.StatusOK, http.StatusPartialContent)
}

func (c *Service) FilesContent(ctx context.Context, req *FilesContentRequest, opts *RequestOptions) (*FilesContentResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	query, err := fileQuery(req.Path, opts)
	if err != nil {
		return nil, err
	}
	if req.MaxTokens != nil {
		query.Set("max_tokens", strconv.Itoa(*req.MaxTokens))
	}

	var resp FilesContentResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/files/content", query, nil, &resp, "", "", opts, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) UploadBytes(ctx context.Context, req *UploadBytesRequest, opts *RequestOptions) ([]RestEntryInfo, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	query, err := fileQuery(req.Path, opts)
	if err != nil {
		return nil, err
	}

	data := req.Data
	requestOpts := cloneOptionsWithHeaders(opts, nil)
	if req.GzipCompress {
		data, err = gzipBytes(req.Data)
		if err != nil {
			return nil, err
		}
		requestOpts = cloneOptionsWithHeaders(requestOpts, http.Header{"Content-Encoding": []string{"gzip"}})
	}

	resp, err := c.do(ctx, http.MethodPost, "/files", query, bytes.NewReader(data), "application/octet-stream", "application/json", requestOpts, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []RestEntryInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Service) UploadJSON(ctx context.Context, entry *WriteFileEntry, opts *RequestOptions) ([]RestEntryInfo, error) {
	if entry == nil || strings.TrimSpace(entry.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp []RestEntryInfo
	if _, err := c.doJSON(ctx, http.MethodPost, "/files", queryFromOptions(opts), entry, &resp, "", "", opts, http.StatusOK); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) UploadMultipart(ctx context.Context, req *UploadMultipartRequest, opts *RequestOptions) ([]RestEntryInfo, error) {
	if req == nil || len(req.Parts) == 0 {
		return nil, ErrMultipartFilesEmpty
	}

	var query url.Values
	var err error
	if strings.TrimSpace(req.Path) != "" {
		query, err = fileQuery(req.Path, opts)
		if err != nil {
			return nil, err
		}
	} else {
		query = queryFromOptions(opts)
	}

	body, contentType, err := writeMultipart(req.Parts)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/files", query, body, contentType, "application/json", opts, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []RestEntryInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Service) WriteBatch(ctx context.Context, req *WriteFilesRequest, opts *RequestOptions) (*WriteFilesBatchResponse, error) {
	var resp WriteFilesBatchResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/files/batch", nil, req, &resp, "", "", opts, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ComposeFiles(ctx context.Context, req *ComposeFilesRequest, opts *RequestOptions) (*RestEntryInfo, error) {
	var resp RestEntryInfo
	if _, err := c.doJSON(ctx, http.MethodPost, "/files/compose", nil, req, &resp, "", "", opts, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListDir(ctx context.Context, req *ListDirRequest, opts *RequestOptions) (*ListDirResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp ListDirResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/ListDir", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) Stat(ctx context.Context, req *StatRequest, opts *RequestOptions) (*StatResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp StatResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/Stat", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) MakeDir(ctx context.Context, req *MakeDirRequest, opts *RequestOptions) (*MakeDirResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp MakeDirResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/MakeDir", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) Remove(ctx context.Context, req *RemoveRequest, opts *RequestOptions) error {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return ErrPathEmpty
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/Remove", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) Move(ctx context.Context, req *MoveRequest, opts *RequestOptions) (*MoveResponse, error) {
	if req == nil || strings.TrimSpace(req.Source) == "" || strings.TrimSpace(req.Destination) == "" {
		return nil, ErrPathEmpty
	}
	var resp MoveResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/Move", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) Edit(ctx context.Context, req *FsEditRequest, opts *RequestOptions) (*FsEditResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp FsEditResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/Edit", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) WatchDir(ctx context.Context, req *WatchDirRequest, opts *RequestOptions) (*FilesystemWatchStream, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	stream, err := c.connectStream(ctx, "/filesystem.Filesystem/WatchDir", req, opts)
	if err != nil {
		return nil, err
	}
	return &FilesystemWatchStream{ConnectStream: stream}, nil
}

func (c *Service) CreateWatcher(ctx context.Context, req *CreateWatcherRequest, opts *RequestOptions) (*CreateWatcherResponse, error) {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return nil, ErrPathEmpty
	}
	var resp CreateWatcherResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/CreateWatcher", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetWatcherEvents(ctx context.Context, req *GetWatcherEventsRequest, opts *RequestOptions) (*GetWatcherEventsResponse, error) {
	if req == nil || strings.TrimSpace(req.WatcherID) == "" {
		return nil, ErrWatcherIDEmpty
	}
	var resp GetWatcherEventsResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/GetWatcherEvents", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) RemoveWatcher(ctx context.Context, req *RemoveWatcherRequest, opts *RequestOptions) error {
	if req == nil || strings.TrimSpace(req.WatcherID) == "" {
		return ErrWatcherIDEmpty
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/filesystem.Filesystem/RemoveWatcher", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) Start(ctx context.Context, req *ProcessStartRequest, opts *RequestOptions) (*ProcessStream, error) {
	if req == nil || req.Process == nil {
		return nil, ErrProcessNil
	}
	if strings.TrimSpace(req.Process.Cmd) == "" {
		return nil, ErrCommandEmpty
	}
	stream, err := c.connectStream(ctx, "/process.Process/Start", req, opts)
	if err != nil {
		return nil, err
	}
	return &ProcessStream{ConnectStream: stream}, nil
}

func (c *Service) Connect(ctx context.Context, req *ConnectRequest, opts *RequestOptions) (*ProcessStream, error) {
	if req == nil {
		return nil, ErrProcessSelectorEmpty
	}
	if err := req.Process.Validate(); err != nil {
		return nil, err
	}
	stream, err := c.connectStream(ctx, "/process.Process/Connect", req, opts)
	if err != nil {
		return nil, err
	}
	return &ProcessStream{ConnectStream: stream}, nil
}

func (c *Service) ListProcesses(ctx context.Context, opts *RequestOptions) (*ProcessListResponse, error) {
	var resp ProcessListResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/process.Process/List", nil, map[string]any{}, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) SendInput(ctx context.Context, req *SendInputRequest, opts *RequestOptions) error {
	if req == nil {
		return ErrProcessSelectorEmpty
	}
	if err := req.Process.Validate(); err != nil {
		return err
	}
	if err := req.Input.Validate(); err != nil {
		return err
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/process.Process/SendInput", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) SendSignal(ctx context.Context, req *SendSignalRequest, opts *RequestOptions) error {
	if req == nil {
		return ErrProcessSelectorEmpty
	}
	if err := req.Process.Validate(); err != nil {
		return err
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/process.Process/SendSignal", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) CloseStdin(ctx context.Context, req *CloseStdinRequest, opts *RequestOptions) error {
	if req == nil {
		return ErrProcessSelectorEmpty
	}
	if err := req.Process.Validate(); err != nil {
		return err
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/process.Process/CloseStdin", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) Update(ctx context.Context, req *UpdateRequest, opts *RequestOptions) error {
	if req == nil {
		return ErrProcessSelectorEmpty
	}
	if err := req.Process.Validate(); err != nil {
		return err
	}
	if req.PTY == nil {
		return ErrPTYRequired
	}
	resp, err := c.doJSON(ctx, http.MethodPost, "/process.Process/Update", nil, req, nil, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (c *Service) StreamInput(ctx context.Context, frames []StreamInputFrame, opts *RequestOptions) (*ConnectFrame, error) {
	if len(frames) == 0 {
		return nil, ErrStreamInputFramesZero
	}
	payload, err := encodeConnectFrames(frames)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(
		ctx,
		http.MethodPost,
		"/process.Process/StreamInput",
		nil,
		bytes.NewReader(payload),
		"application/connect+json",
		"application/connect+json",
		withBasicUsername(withConnectRPC(opts)),
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	stream := &ConnectStream{resp: resp}
	return stream.NextFrame()
}

func (c *Service) GetResult(ctx context.Context, req *GetResultRequest, opts *RequestOptions) (*GetResultResponse, error) {
	if req == nil || strings.TrimSpace(req.CmdID) == "" {
		return nil, ErrCmdIDEmpty
	}
	var resp GetResultResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/process.Process/GetResult", nil, req, &resp, "", "", withBasicUsername(withConnectRPC(opts)), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) Run(ctx context.Context, req *AgentRunRequest, opts *RequestOptions) (*AgentRunResponse, error) {
	if req == nil || strings.TrimSpace(req.Cmd) == "" {
		return nil, ErrCommandEmpty
	}
	var resp AgentRunResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/run", nil, req, &resp, "", "", withBasicUsername(opts), http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ReadFile(ctx context.Context, req *FileRequest, opts *RequestOptions) (*http.Response, error) {
	if req == nil {
		return nil, ErrPathEmpty
	}
	query, err := fileQuery(req.Path, opts)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, "/file", query, nil, "", "*/*", opts, http.StatusOK)
}

func (c *Service) WriteFile(ctx context.Context, req *UploadBytesRequest, opts *RequestOptions) error {
	if req == nil || strings.TrimSpace(req.Path) == "" {
		return ErrPathEmpty
	}
	query, err := fileQuery(req.Path, opts)
	if err != nil {
		return err
	}
	data := req.Data
	requestOpts := cloneOptionsWithHeaders(opts, nil)
	if req.GzipCompress {
		data, err = gzipBytes(req.Data)
		if err != nil {
			return err
		}
		requestOpts = cloneOptionsWithHeaders(requestOpts, http.Header{"Content-Encoding": []string{"gzip"}})
	}
	resp, err := c.do(ctx, http.MethodPost, "/file", query, bytes.NewReader(data), "application/octet-stream", "", requestOpts, http.StatusNoContent)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (s ProcessSelector) Validate() error {
	hasPID := s.PID != 0
	hasTag := strings.TrimSpace(s.Tag) != ""
	switch {
	case !hasPID && !hasTag:
		return ErrProcessSelectorEmpty
	case hasPID && hasTag:
		return ErrProcessSelectorAmbig
	default:
		return nil
	}
}

func (i ProcessInput) Validate() error {
	if strings.TrimSpace(i.Stdin) == "" && strings.TrimSpace(i.PTY) == "" {
		return ErrProcessInputEmpty
	}
	return nil
}
