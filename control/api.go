package control

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/SeaArt-Infra/sandbox-go/core"
)

const (
	logsDirectionForward  = "forward"
	logsDirectionBackward = "backward"
	maxExtendSeconds      = 86400
	maxRefreshSeconds     = 3600
	maxLogsLimit          = 1000
	maxLogsSearchLength   = 256
)

type wrappedResponse[T any] struct {
	Code      int               `json:"code"`
	Message   string            `json:"message"`
	Data      T                 `json:"data"`
	Err       *core.ErrorDetail `json:"error,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func (c *Service) Metrics(ctx context.Context) (string, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/metrics", nil, nil, nil, http.StatusOK)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Service) Shutdown(ctx context.Context) (*ShutdownResponse, error) {
	var resp ShutdownResponse
	if _, err := c.DoJSON(ctx, http.MethodPost, "/shutdown", nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) CreateSandbox(ctx context.Context, req *NewSandboxRequest) (*Sandbox, error) {
	if req == nil {
		return nil, ErrTemplateEmpty
	}

	var resp Sandbox
	if _, err := c.DoJSON(ctx, http.MethodPost, "/api/v1/sandboxes", nil, nil, req, &resp, http.StatusCreated); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListSandboxes(ctx context.Context, params *ListSandboxesParams) ([]ListedSandbox, error) {
	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp []ListedSandbox
	if _, err := c.DoJSON(ctx, http.MethodGet, "/api/v1/sandboxes", nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) GetSandbox(ctx context.Context, sandboxID string) (*SandboxDetail, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return nil, ErrSandboxIDEmpty
	}

	var resp SandboxDetail
	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) DeleteSandbox(ctx context.Context, sandboxID string) error {
	if strings.TrimSpace(sandboxID) == "" {
		return ErrSandboxIDEmpty
	}

	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID)
	_, err := c.DoRequest(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent)
	return err
}

func (c *Service) GetSandboxLogs(ctx context.Context, sandboxID string, params *SandboxLogsParams) (*SandboxLogsResponse, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return nil, ErrSandboxIDEmpty
	}
	if err := validateSandboxLogsParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp SandboxLogsResponse
	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/logs"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) PauseSandbox(ctx context.Context, sandboxID string) error {
	if strings.TrimSpace(sandboxID) == "" {
		return ErrSandboxIDEmpty
	}

	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/pause"
	_, err := c.DoRequest(ctx, http.MethodPost, path, nil, nil, nil, http.StatusNoContent)
	return err
}

func (c *Service) ConnectSandbox(ctx context.Context, sandboxID string, req *ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return nil, ErrSandboxIDEmpty
	}
	if req == nil {
		return nil, fmt.Errorf("sandbox: connect request is required")
	}
	if err := validateTimeoutSeconds(req.Timeout, "connect timeout"); err != nil {
		return nil, err
	}

	var resp Sandbox
	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/connect"
	httpResp, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, req, &resp, http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	return &ConnectSandboxResponse{
		StatusCode: httpResp.StatusCode,
		Sandbox:    &resp,
	}, nil
}

func (c *Service) SetSandboxTimeout(ctx context.Context, sandboxID string, req *TimeoutRequest) error {
	if strings.TrimSpace(sandboxID) == "" {
		return ErrSandboxIDEmpty
	}
	if req == nil {
		return fmt.Errorf("sandbox: timeout request is required")
	}
	if err := validateTimeoutSeconds(req.Timeout, "timeout"); err != nil {
		return err
	}

	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/timeout"
	_, err := c.DoRequest(ctx, http.MethodPost, path, nil, nil, req, http.StatusNoContent)
	return err
}

func (c *Service) RefreshSandbox(ctx context.Context, sandboxID string, req *RefreshSandboxRequest) error {
	if strings.TrimSpace(sandboxID) == "" {
		return ErrSandboxIDEmpty
	}
	if err := validateRefreshRequest(req); err != nil {
		return err
	}

	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/refreshes"
	var body any
	if req != nil {
		body = req
	}
	_, err := c.DoRequest(ctx, http.MethodPost, path, nil, nil, body, http.StatusNoContent)
	return err
}

func (c *Service) SendHeartbeat(ctx context.Context, sandboxID string, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return nil, ErrSandboxIDEmpty
	}
	if req == nil {
		return nil, fmt.Errorf("sandbox: heartbeat request is required")
	}
	if err := validateHeartbeatStatus(req.Status); err != nil {
		return nil, err
	}

	var wrapped wrappedResponse[HeartbeatResponse]
	path := "/api/v1/sandboxes/" + url.PathEscape(sandboxID) + "/heartbeat"
	if _, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, req, &wrapped, http.StatusOK); err != nil {
		return nil, err
	}
	wrapped.Data.RequestID = wrapped.RequestID
	return &wrapped.Data, nil
}

func (c *Service) GetPoolStatus(ctx context.Context) (*PoolStatus, error) {
	var wrapped wrappedResponse[PoolStatus]
	if _, err := c.DoJSON(ctx, http.MethodGet, "/admin/pool/status", nil, nil, nil, &wrapped, http.StatusOK); err != nil {
		return nil, err
	}
	wrapped.Data.RequestID = wrapped.RequestID
	return &wrapped.Data, nil
}

func (c *Service) StartRollingUpdate(ctx context.Context, req *RollingStartRequest) (*RollingUpdateStatus, error) {
	if req == nil || strings.TrimSpace(req.TemplateID) == "" {
		return nil, ErrTemplateEmpty
	}

	var wrapped wrappedResponse[RollingUpdateStatus]
	if _, err := c.DoJSON(ctx, http.MethodPost, "/admin/rolling/start", nil, nil, req, &wrapped, http.StatusOK); err != nil {
		return nil, err
	}
	wrapped.Data.RequestID = wrapped.RequestID
	return &wrapped.Data, nil
}

func (c *Service) GetRollingUpdateStatus(ctx context.Context) (*RollingUpdateStatus, error) {
	var wrapped wrappedResponse[RollingUpdateStatus]
	if _, err := c.DoJSON(ctx, http.MethodGet, "/admin/rolling/status", nil, nil, nil, &wrapped, http.StatusOK); err != nil {
		return nil, err
	}
	wrapped.Data.RequestID = wrapped.RequestID
	return &wrapped.Data, nil
}

func (c *Service) CancelRollingUpdate(ctx context.Context) (*RollingUpdateStatus, error) {
	var wrapped wrappedResponse[RollingUpdateStatus]
	if _, err := c.DoJSON(ctx, http.MethodPost, "/admin/rolling/cancel", nil, nil, nil, &wrapped, http.StatusOK); err != nil {
		return nil, err
	}
	wrapped.Data.RequestID = wrapped.RequestID
	return &wrapped.Data, nil
}

func (p *ListSandboxesParams) encode() url.Values {
	values := make(url.Values)
	if len(p.Metadata) > 0 {
		metadata := make(url.Values, len(p.Metadata))
		for key, value := range p.Metadata {
			metadata.Set(key, value)
		}
		values.Set("metadata", metadata.Encode())
	}
	for _, state := range p.State {
		if s := strings.TrimSpace(state); s != "" {
			values.Add("state", s)
		}
	}
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if token := strings.TrimSpace(p.NextToken); token != "" {
		values.Set("nextToken", token)
	}
	return values
}

func (p *SandboxLogsParams) encode() url.Values {
	values := make(url.Values)
	if p.Cursor != nil {
		values.Set("cursor", strconv.FormatInt(*p.Cursor, 10))
	}
	if p.Limit != nil {
		values.Set("limit", strconv.Itoa(*p.Limit))
	}
	if direction := strings.TrimSpace(p.Direction); direction != "" {
		values.Set("direction", direction)
	}
	if level := strings.TrimSpace(p.Level); level != "" {
		values.Set("level", level)
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		values.Set("search", search)
	}
	return values
}

func validateTimeoutSeconds(timeout int32, field string) error {
	if timeout < 0 || timeout > maxExtendSeconds {
		return fmt.Errorf("sandbox: %s must be between 0 and %d", field, maxExtendSeconds)
	}
	return nil
}

func validateRefreshRequest(req *RefreshSandboxRequest) error {
	if req == nil || req.Duration == nil {
		return nil
	}
	if *req.Duration < 0 || *req.Duration > maxRefreshSeconds {
		return fmt.Errorf("sandbox: refresh duration must be between 0 and %d", maxRefreshSeconds)
	}
	return nil
}

func validateHeartbeatStatus(status string) error {
	switch strings.TrimSpace(status) {
	case "starting", "healthy", "error":
		return nil
	default:
		return fmt.Errorf("sandbox: heartbeat status must be one of starting, healthy, error")
	}
}

func validateSandboxLogsParams(params *SandboxLogsParams) error {
	if params == nil {
		return nil
	}
	if params.Cursor != nil && *params.Cursor < 0 {
		return fmt.Errorf("sandbox: logs cursor must be non-negative")
	}
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > maxLogsLimit) {
		return fmt.Errorf("sandbox: logs limit must be between 0 and %d", maxLogsLimit)
	}
	if direction := strings.TrimSpace(params.Direction); direction != "" &&
		direction != logsDirectionForward && direction != logsDirectionBackward {
		return fmt.Errorf("sandbox: logs direction must be %q or %q", logsDirectionForward, logsDirectionBackward)
	}
	if len(params.Search) > maxLogsSearchLength {
		return fmt.Errorf("sandbox: logs search must be at most %d characters", maxLogsSearchLength)
	}
	return nil
}
