package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

type Sandbox struct {
	*control.Sandbox
	client  *Client
	runtime *Runtime
}

type SandboxDetail struct {
	*control.SandboxDetail
	client  *Client
	runtime *Runtime
}

type SandboxHandle struct {
	*control.ListedSandbox
	client *Client
}

type ConnectSandboxResponse struct {
	StatusCode int
	Sandbox    *Sandbox
}

func (c *Client) CreateSandbox(ctx context.Context, req *control.NewSandboxRequest) (*Sandbox, error) {
	if req == nil {
		return nil, core.ErrTemplateEmpty
	}
	waitReady := req.WaitReady != nil && *req.WaitReady
	request := *req
	if waitReady {
		wait := false
		request.WaitReady = &wait
	}
	created, err := c.Service.CreateSandbox(ctx, &request)
	if err != nil {
		return nil, err
	}
	sandbox := bindSandbox(c, created)
	if !waitReady || sandboxControlReady(created.Status, created.State) {
		return sandbox, nil
	}
	waitTimeout := req.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = 3 * time.Minute
	}
	pollInterval := req.PollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	for {
		detail, getErr := c.Service.GetSandbox(waitCtx, created.SandboxID)
		if getErr == nil && detail != nil {
			updateCreatedSandbox(created, detail)
			if sandboxControlReady(detail.Status, detail.State) {
				return bindSandbox(c, created), nil
			}
			if sandboxControlTerminalFailure(detail.Status, detail.State) {
				return bindSandbox(c, created), &ResourceOperationError{
					Operation: "wait for sandbox readiness", ResourceType: "sandbox", ResourceID: created.SandboxID,
					Err: fmt.Errorf("sandbox entered terminal state %q", firstNonEmpty(detail.State, detail.Status)),
				}
			}
		} else if getErr != nil {
			var apiErr *core.APIError
			if !errors.As(getErr, &apiErr) || (apiErr.Kind != core.APIErrorKindNotFound && !apiErr.Retryable()) {
				return sandbox, &ResourceOperationError{Operation: "wait for sandbox readiness", ResourceType: "sandbox", ResourceID: created.SandboxID, Err: getErr}
			}
		}
		if pollErr := waitForPoll(waitCtx, pollInterval); pollErr != nil {
			return bindSandbox(c, created), &ResourceOperationError{Operation: "wait for sandbox readiness", ResourceType: "sandbox", ResourceID: created.SandboxID, Err: pollErr}
		}
	}
}

func sandboxControlReady(status, state string) bool {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(state, status))) {
	case "active", "running", "ready":
		return true
	default:
		return false
	}
}

func sandboxControlTerminalFailure(status, state string) bool {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(state, status))) {
	case "deleted", "destroying", "failed", "error", "expired":
		return true
	default:
		return false
	}
}

func updateCreatedSandbox(created *control.Sandbox, detail *control.SandboxDetail) {
	if created == nil || detail == nil {
		return
	}
	created.TemplateID = detail.TemplateID
	created.SandboxID = detail.SandboxID
	created.ClientID = detail.ClientID
	created.EnvdVersion = detail.EnvdVersion
	created.EnvdAccessToken = detail.EnvdAccessToken
	created.EnvdURL = detail.EnvdURL
	created.Namespace = detail.Namespace
	created.Status = detail.Status
	created.State = detail.State
	created.StartedAt = detail.StartedAt
	created.ActivatedAt = detail.ActivatedAt
	created.EndAt = detail.EndAt
}

func (c *Client) GetSandbox(ctx context.Context, sandboxID string) (*SandboxDetail, error) {
	sandbox, err := c.Service.GetSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	return bindSandboxDetail(c, sandbox), nil
}

func (c *Client) ListSandboxes(
	ctx context.Context,
	params *control.ListSandboxesParams,
) ([]*SandboxHandle, error) {
	sandboxes, err := c.Service.ListSandboxes(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]*SandboxHandle, 0, len(sandboxes))
	for i := range sandboxes {
		sandbox := sandboxes[i]
		out = append(out, bindSandboxHandle(c, &sandbox))
	}
	return out, nil
}

func (c *Client) ConnectSandbox(
	ctx context.Context,
	sandboxID string,
	req *control.ConnectSandboxRequest,
) (*ConnectSandboxResponse, error) {
	resp, err := c.Service.ConnectSandbox(ctx, sandboxID, req)
	if err != nil {
		return nil, err
	}
	return &ConnectSandboxResponse{
		StatusCode: resp.StatusCode,
		Sandbox:    bindSandbox(c, resp.Sandbox),
	}, nil
}

func (s *Sandbox) Runtime() (*Runtime, error) {
	if s == nil || s.EnvdURL == nil || strings.TrimSpace(*s.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}
	if s.runtime != nil {
		return s.runtime, nil
	}
	runtime, err := s.client.RuntimeFromSandbox(s.Sandbox)
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	return s.runtime, nil
}

func (s *Sandbox) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox and returns the running sandbox handle.
func (s *Sandbox) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout <= 0 {
		timeout = 300
	}
	resp, err := s.client.ConnectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo fetches the latest sandbox detail for this sandbox ID.
func (s *Sandbox) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

// GetMetrics reads runtime metrics for sandboxes that expose nano-executor access.
func (s *Sandbox) GetMetrics(ctx context.Context) (*cmd.MetricsResponse, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Metrics(ctx)
}

func (s *Sandbox) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.client.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *Sandbox) Pause(ctx context.Context) error {
	return s.client.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	return s.Delete(ctx)
}

func (s *Sandbox) Delete(ctx context.Context) error {
	return s.client.DeleteSandbox(ctx, s.SandboxID)
}

func (s *Sandbox) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.client.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *Sandbox) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.client.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *Sandbox) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.client.ConnectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *Sandbox) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func (s *SandboxDetail) Runtime() (*Runtime, error) {
	if s == nil || s.EnvdURL == nil || strings.TrimSpace(*s.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}
	if s.runtime != nil {
		return s.runtime, nil
	}
	runtime, err := s.client.RuntimeFromDetail(s.SandboxDetail)
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	return s.runtime, nil
}

func (s *SandboxDetail) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox detail and returns a running sandbox handle.
func (s *SandboxDetail) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout <= 0 {
		timeout = 300
	}
	resp, err := s.client.ConnectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo refreshes the sandbox detail for this sandbox ID.
func (s *SandboxDetail) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

// GetMetrics reads runtime metrics for sandboxes that expose nano-executor access.
func (s *SandboxDetail) GetMetrics(ctx context.Context) (*cmd.MetricsResponse, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Metrics(ctx)
}

func (s *SandboxDetail) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.client.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *SandboxDetail) Pause(ctx context.Context) error {
	return s.client.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *SandboxDetail) Kill(ctx context.Context) error {
	return s.Delete(ctx)
}

func (s *SandboxDetail) Delete(ctx context.Context) error {
	return s.client.DeleteSandbox(ctx, s.SandboxID)
}

func (s *SandboxDetail) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.client.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *SandboxDetail) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.client.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *SandboxDetail) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.client.ConnectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *SandboxDetail) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func (s *SandboxHandle) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox handle and returns a running sandbox handle.
func (s *SandboxHandle) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout <= 0 {
		timeout = 300
	}
	resp, err := s.client.ConnectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo fetches the latest sandbox detail for this sandbox ID.
func (s *SandboxHandle) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.client.GetSandbox(ctx, s.SandboxID)
}

func (s *SandboxHandle) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.client.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *SandboxHandle) Pause(ctx context.Context) error {
	return s.client.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *SandboxHandle) Kill(ctx context.Context) error {
	return s.Delete(ctx)
}

func (s *SandboxHandle) Delete(ctx context.Context) error {
	return s.client.DeleteSandbox(ctx, s.SandboxID)
}

func (s *SandboxHandle) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.client.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *SandboxHandle) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.client.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *SandboxHandle) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.client.ConnectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *SandboxHandle) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func bindSandbox(client *Client, sandbox *control.Sandbox) *Sandbox {
	if sandbox == nil {
		return nil
	}
	return &Sandbox{
		Sandbox: sandbox,
		client:  client,
	}
}

func bindSandboxDetail(client *Client, sandbox *control.SandboxDetail) *SandboxDetail {
	if sandbox == nil {
		return nil
	}
	return &SandboxDetail{
		SandboxDetail: sandbox,
		client:        client,
	}
}

func bindSandboxHandle(client *Client, sandbox *control.ListedSandbox) *SandboxHandle {
	if sandbox == nil {
		return nil
	}
	return &SandboxHandle{
		ListedSandbox: sandbox,
		client:        client,
	}
}

func isRunningSandboxState(state, status string) bool {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(state, status))) {
	case "paused", "stopped", "deleted":
		return false
	default:
		return true
	}
}
