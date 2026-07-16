package sandbox

import (
	"context"

	"github.com/SeaArt-Infra/sandbox-go/build"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

type Client struct {
	*control.Service
	Build *build.Service
}

func NewClient(baseURL, apiKey string, opts ...core.TransportOption) (*Client, error) {
	controlService, err := control.NewService(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	buildOps, err := build.NewService(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		Service: controlService,
		Build:   buildOps,
	}, nil
}

func (c *Client) NewCMD(baseURL, accessToken string) (*cmd.Service, error) {
	return cmd.NewService(baseURL, accessToken)
}

func (c *Client) Runtime(baseURL, accessToken string) (*Runtime, error) {
	service, err := cmd.NewService(baseURL, accessToken)
	if err != nil {
		return nil, err
	}
	return &Runtime{Service: service}, nil
}

func (c *Client) Create(ctx context.Context, templateID string, opts *CreateOptions) (*Sandbox, error) {
	req := &control.NewSandboxRequest{TemplateID: templateID}
	if opts != nil {
		req.TemplateID = firstNonEmpty(templateID, opts.TemplateID)
		req.WorkspaceID = opts.WorkspaceID
		req.Timeout = opts.Timeout
		req.Metadata = opts.Metadata
		req.EnvVars = opts.EnvVars
		req.VolumeMounts = opts.VolumeMounts
		req.WaitReady = opts.WaitReady
		req.AutoPause = opts.AutoPause
		req.WaitTimeout = opts.WaitTimeout
		req.PollInterval = opts.PollInterval
	}
	return c.CreateSandbox(ctx, req)
}

func (c *Client) Connect(ctx context.Context, sandboxID string, opts *ConnectOptions) (*Sandbox, error) {
	timeout := int32(300)
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	resp, err := c.ConnectSandbox(ctx, sandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

func (c *Client) List(ctx context.Context, opts *ListOptions) ([]*SandboxHandle, error) {
	if opts == nil {
		return c.ListSandboxes(ctx, nil)
	}
	return c.ListSandboxes(ctx, &control.ListSandboxesParams{
		Metadata:  opts.Metadata,
		State:     opts.State,
		Limit:     opts.Limit,
		NextToken: opts.NextToken,
	})
}

func (c *Client) Get(ctx context.Context, sandboxID string) (*SandboxDetail, error) {
	return c.GetSandbox(ctx, sandboxID)
}

func (c *Client) BuildTemplate(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions) (*TemplateBuildInfo, error) {
	return buildTemplateWithService(ctx, c.Build, template, name, cloneTemplateBuildOptions(opts))
}

func (c *Client) BuildTemplateInBackground(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions) (*TemplateBuildInfo, error) {
	cloned := cloneTemplateBuildOptions(opts)
	wait := false
	cloned.Wait = &wait
	return buildTemplateWithService(ctx, c.Build, template, name, cloned)
}

func (c *Client) ListTemplates(ctx context.Context, opts *TemplateListOptions) ([]build.ListedTemplate, error) {
	return listTemplatesWithService(ctx, c.Build, cloneTemplateListOptions(opts))
}

func (c *Client) GetTemplate(ctx context.Context, ref string, opts *TemplateGetOptions) (*build.TemplateResponse, error) {
	return getTemplateWithService(ctx, c.Build, ref, cloneTemplateGetOptions(opts))
}

func (c *Client) DeleteTemplate(ctx context.Context, ref string) error {
	return deleteTemplateWithService(ctx, c.Build, ref)
}

func (c *Client) TemplateExists(ctx context.Context, ref string) (bool, error) {
	return templateExistsWithService(ctx, c.Build, ref)
}

func (c *Client) TemplateAliasExists(ctx context.Context, alias string) (bool, error) {
	return c.TemplateExists(ctx, alias)
}

func (c *Client) GetTemplateBuildStatus(ctx context.Context, templateID, buildID string, opts *TemplateBuildStatusOptions) (*build.BuildStatusResponse, error) {
	return getTemplateBuildStatusWithService(ctx, c.Build, templateID, buildID, cloneTemplateBuildStatusOptions(opts))
}

func cloneTemplateBuildOptions(opts *TemplateBuildOptions) *TemplateBuildOptions {
	if opts == nil {
		return &TemplateBuildOptions{}
	}
	cloned := *opts
	cloned.GatewayConfig = GatewayConfig{}
	return &cloned
}

func cloneTemplateListOptions(opts *TemplateListOptions) *TemplateListOptions {
	if opts == nil {
		return &TemplateListOptions{}
	}
	cloned := *opts
	cloned.GatewayConfig = GatewayConfig{}
	return &cloned
}

func cloneTemplateGetOptions(opts *TemplateGetOptions) *TemplateGetOptions {
	if opts == nil {
		return &TemplateGetOptions{}
	}
	cloned := *opts
	cloned.GatewayConfig = GatewayConfig{}
	return &cloned
}

func cloneTemplateBuildStatusOptions(opts *TemplateBuildStatusOptions) *TemplateBuildStatusOptions {
	if opts == nil {
		return &TemplateBuildStatusOptions{}
	}
	cloned := *opts
	cloned.GatewayConfig = GatewayConfig{}
	return &cloned
}
