package cmd

import (
	"errors"

	"github.com/SeaArt-Infra/sandbox-go/core"
)

var (
	ErrBaseURLEmpty          = core.ErrBaseURLEmpty
	ErrInvalidBaseURL        = core.ErrInvalidBaseURL
	ErrPathEmpty             = errors.New("sandbox/cmd: path is required")
	ErrCommandEmpty          = errors.New("sandbox/cmd: cmd is required")
	ErrProcessNil            = errors.New("sandbox/cmd: process is required")
	ErrWatcherIDEmpty        = errors.New("sandbox/cmd: watcherID is required")
	ErrCmdIDEmpty            = errors.New("sandbox/cmd: cmdID is required")
	ErrPortInvalid           = errors.New("sandbox/cmd: port must be greater than 0")
	ErrProcessSelectorEmpty  = errors.New("sandbox/cmd: process selector requires pid or tag")
	ErrProcessSelectorAmbig  = errors.New("sandbox/cmd: process selector requires exactly one of pid or tag")
	ErrProcessInputEmpty     = errors.New("sandbox/cmd: process input requires stdin or pty")
	ErrPTYRequired           = errors.New("sandbox/cmd: pty is required")
	ErrMultipartFilesEmpty   = errors.New("sandbox/cmd: multipart upload requires at least one part")
	ErrStreamInputFramesZero = errors.New("sandbox/cmd: stream input requires at least one frame")
)
