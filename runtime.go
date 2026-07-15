package sandbox

import (
	"strings"

	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

type Runtime struct {
	*cmd.Service
}

func (c *Client) RuntimeFromSandbox(sandbox *control.Sandbox) (*Runtime, error) {
	if sandbox == nil || sandbox.EnvdURL == nil || strings.TrimSpace(*sandbox.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}

	accessToken := ""
	if sandbox.EnvdAccessToken != nil {
		accessToken = *sandbox.EnvdAccessToken
	}
	return c.Runtime(*sandbox.EnvdURL, accessToken)
}

func (c *Client) RuntimeFromDetail(sandbox *control.SandboxDetail) (*Runtime, error) {
	if sandbox == nil || sandbox.EnvdURL == nil || strings.TrimSpace(*sandbox.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}

	accessToken := ""
	if sandbox.EnvdAccessToken != nil {
		accessToken = *sandbox.EnvdAccessToken
	}
	return c.Runtime(*sandbox.EnvdURL, accessToken)
}
