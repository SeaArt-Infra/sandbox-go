package build

import (
	"errors"

	"github.com/SeaArt-Infra/sandbox-go/core"
)

var (
	ErrBaseURLEmpty   = core.ErrBaseURLEmpty
	ErrAPIKeyEmpty    = core.ErrAPIKeyEmpty
	ErrInvalidBaseURL = core.ErrInvalidBaseURL
	ErrNamespaceEmpty = core.ErrNamespaceEmpty
	ErrUserIDEmpty    = core.ErrUserIDEmpty
	ErrTemplateEmpty  = core.ErrTemplateEmpty

	ErrAliasEmpty = errors.New("sandbox: alias is required")
	ErrBuildEmpty = errors.New("sandbox: buildID is required")
	ErrHashEmpty  = errors.New("sandbox: hash is required")
)
