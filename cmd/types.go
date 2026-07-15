package cmd

import (
	"io"
	"net/http"
)

type FileType string

const (
	FileTypeUnspecified FileType = "FILE_TYPE_UNSPECIFIED"
	FileTypeFile        FileType = "FILE_TYPE_FILE"
	FileTypeDirectory   FileType = "FILE_TYPE_DIRECTORY"
	FileTypeSymlink     FileType = "FILE_TYPE_SYMLINK"
)

type EventType string

const (
	EventTypeUnspecified EventType = "EVENT_TYPE_UNSPECIFIED"
	EventTypeCreate      EventType = "EVENT_TYPE_CREATE"
	EventTypeWrite       EventType = "EVENT_TYPE_WRITE"
	EventTypeRemove      EventType = "EVENT_TYPE_REMOVE"
	EventTypeRename      EventType = "EVENT_TYPE_RENAME"
	EventTypeChmod       EventType = "EVENT_TYPE_CHMOD"
)

type Signal string

const (
	SignalUnspecified Signal = "SIGNAL_UNSPECIFIED"
	SignalSIGTERM     Signal = "SIGNAL_SIGTERM"
	SignalSIGKILL     Signal = "SIGNAL_SIGKILL"
	SignalSIGINT      Signal = "SIGINT"
	SignalSIGHUP      Signal = "SIGHUP"
)

type RequestOptions struct {
	Username            string
	Signature           string
	SignatureExpiration *int64
	Range               string
	Headers             http.Header
}

type EntryInfo struct {
	Name          string   `json:"name"`
	Type          FileType `json:"type"`
	Path          string   `json:"path"`
	Size          int64    `json:"size"`
	Mode          int64    `json:"mode"`
	Permissions   string   `json:"permissions"`
	Owner         string   `json:"owner"`
	Group         string   `json:"group"`
	ModifiedTime  string   `json:"modifiedTime"`
	SymlinkTarget *string  `json:"symlinkTarget,omitempty"`
}

type FilesystemEvent struct {
	Name string    `json:"name"`
	Type EventType `json:"type"`
}

type ProcessSelector struct {
	PID int    `json:"pid,omitempty"`
	Tag string `json:"tag,omitempty"`
}

type ProcessInput struct {
	Stdin string `json:"stdin,omitempty"`
	PTY   string `json:"pty,omitempty"`
}

type ProcessConfig struct {
	Cmd  string            `json:"cmd"`
	Args []string          `json:"args,omitempty"`
	Envs map[string]string `json:"envs,omitempty"`
	CWD  *string           `json:"cwd,omitempty"`
}

type PtySize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type PtyConfig struct {
	Size PtySize `json:"size"`
}

type ListDirRequest struct {
	Path  string `json:"path"`
	Depth *int   `json:"depth,omitempty"`
}

type ListDirResponse struct {
	Entries []EntryInfo `json:"entries"`
}

type StatRequest struct {
	Path string `json:"path"`
}

type StatResponse struct {
	Entry EntryInfo `json:"entry"`
}

type MakeDirRequest struct {
	Path string `json:"path"`
}

type MakeDirResponse struct {
	Entry EntryInfo `json:"entry"`
}

type RemoveRequest struct {
	Path string `json:"path"`
}

type MoveRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type MoveResponse struct {
	Entry EntryInfo `json:"entry"`
}

type FsEditRequest struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

type FsEditResponse struct {
	Message string `json:"message"`
}

type WatchDirRequest struct {
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

type FilesystemWatchFrame struct {
	Start      *struct{}        `json:"start,omitempty"`
	Keepalive  *struct{}        `json:"keepalive,omitempty"`
	Filesystem *FilesystemEvent `json:"filesystem,omitempty"`
}

type CreateWatcherRequest struct {
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

type CreateWatcherResponse struct {
	WatcherID string `json:"watcherId"`
}

type GetWatcherEventsRequest struct {
	WatcherID string `json:"watcherId"`
	Limit     *int   `json:"limit,omitempty"`
}

type GetWatcherEventsResponse struct {
	Events []FilesystemEvent `json:"events"`
}

type RemoveWatcherRequest struct {
	WatcherID string `json:"watcherId"`
}

type ProcessStartRequest struct {
	Process *ProcessConfig `json:"process"`
	Timeout *int           `json:"timeout,omitempty"`
	Tag     string         `json:"tag,omitempty"`
	Stdin   *bool          `json:"stdin,omitempty"`
	PTY     *PtyConfig     `json:"pty,omitempty"`
}

type ConnectRequest struct {
	Process ProcessSelector `json:"process"`
}

type SendInputRequest struct {
	Process ProcessSelector `json:"process"`
	Input   ProcessInput    `json:"input"`
}

type SendSignalRequest struct {
	Process ProcessSelector `json:"process"`
	Signal  string          `json:"signal"`
}

type CloseStdinRequest struct {
	Process ProcessSelector `json:"process"`
}

type UpdateRequest struct {
	Process ProcessSelector `json:"process"`
	PTY     *PtyConfig      `json:"pty"`
}

type ProcessInfo struct {
	PID    int           `json:"pid"`
	Config ProcessConfig `json:"config"`
	Tag    string        `json:"tag,omitempty"`
	CmdID  string        `json:"cmdId,omitempty"`
}

type ProcessListResponse struct {
	Processes []ProcessInfo `json:"processes"`
}

type GetResultRequest struct {
	CmdID string `json:"cmdId"`
}

type GetResultResponse struct {
	ExitCode      int    `json:"exitCode"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	StartedAtUnix int64  `json:"startedAtUnix"`
}

type ProcessStartEvent struct {
	PID   int    `json:"pid"`
	CmdID string `json:"cmdId"`
}

type ProcessDataEvent struct {
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	PTY    string `json:"pty,omitempty"`
}

type ProcessEndEvent struct {
	Exited bool    `json:"exited"`
	Status string  `json:"status"`
	Error  *string `json:"error"`
}

type ProcessEvent struct {
	Start     *ProcessStartEvent `json:"start,omitempty"`
	Data      *ProcessDataEvent  `json:"data,omitempty"`
	End       *ProcessEndEvent   `json:"end,omitempty"`
	Keepalive *struct{}          `json:"keepalive,omitempty"`
}

type ProcessStreamFrame struct {
	Event ProcessEvent `json:"event"`
}

type StreamInputStart struct {
	Process ProcessSelector `json:"process"`
}

type StreamInputData struct {
	Input ProcessInput `json:"input"`
}

type StreamInputFrame struct {
	Start     *StreamInputStart `json:"start,omitempty"`
	Data      *StreamInputData  `json:"data,omitempty"`
	Keepalive *struct{}         `json:"keepalive,omitempty"`
}

type RestEntryType string

const (
	RestEntryTypeFile      RestEntryType = "file"
	RestEntryTypeDirectory RestEntryType = "directory"
)

type RestEntryInfo struct {
	Path string        `json:"path"`
	Name string        `json:"name"`
	Type RestEntryType `json:"type"`
}

type WriteFileEntry struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Data    string `json:"data,omitempty"`
	Mode    *int   `json:"mode,omitempty"`
}

type WriteFilesRequest struct {
	Files []WriteFileEntry `json:"files"`
}

type WriteFilesBatchResult struct {
	Path         string `json:"path"`
	BytesWritten int64  `json:"bytes_written"`
}

type WriteFilesBatchResponse struct {
	Files []WriteFilesBatchResult `json:"files"`
}

type ComposeFilesRequest struct {
	SourcePaths []string `json:"source_paths"`
	Destination string   `json:"destination"`
}

type FilesContentResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	MIMEType  string `json:"mime_type,omitempty"`
	Data      string `json:"data,omitempty"`
}

type PortEntry struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Address     string `json:"address"`
	PID         *int   `json:"pid,omitempty"`
	ProcessName string `json:"process_name,omitempty"`
}

type MetricsResponse struct {
	TS          int64   `json:"ts"`
	CPUCount    int     `json:"cpu_count"`
	CPUUsedPct  float64 `json:"cpu_used_pct"`
	MemTotal    int64   `json:"mem_total"`
	MemUsed     int64   `json:"mem_used"`
	MemTotalMiB int64   `json:"mem_total_mib"`
	MemUsedMiB  int64   `json:"mem_used_mib"`
	MemCache    int64   `json:"mem_cache"`
	DiskUsed    int64   `json:"disk_used"`
	DiskTotal   int64   `json:"disk_total"`
}

type ConfigureRequest struct {
	Envs map[string]string `json:"envs,omitempty"`
}

type AgentRunRequest struct {
	Cmd     string            `json:"cmd"`
	Args    []string          `json:"args,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout *int              `json:"timeout,omitempty"`
	Stdin   *string           `json:"stdin,omitempty"`
}

type AgentRunResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type DownloadRequest struct {
	Path string
}

type FilesContentRequest struct {
	Path      string
	MaxTokens *int
}

type UploadBytesRequest struct {
	Path         string
	Data         []byte
	GzipCompress bool
}

type MultipartFile struct {
	FieldName   string
	FileName    string
	ContentType string
	Data        []byte
}

type UploadMultipartRequest struct {
	Path  string
	Parts []MultipartFile
}

type FileRequest struct {
	Path string
}

type ProxyRequest struct {
	Method  string
	Port    int
	Path    string
	Body    io.Reader
	Headers http.Header
}
