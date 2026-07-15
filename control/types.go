package control

import "time"

// ShutdownResponse is returned by POST /shutdown.
type ShutdownResponse struct {
	Message string `json:"message"`
}

// VolumeMount represents one extra sandbox volume mount.
type VolumeMount struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// NewSandboxRequest is the request body for creating a sandbox.
type NewSandboxRequest struct {
	TemplateID   string            `json:"templateID,omitempty"`
	WorkspaceID  string            `json:"workspaceId,omitempty"`
	Timeout      *int32            `json:"timeout,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	EnvVars      map[string]string `json:"envVars,omitempty"`
	VolumeMounts []VolumeMount     `json:"volumeMounts,omitempty"`
	WaitReady    *bool             `json:"waitReady,omitempty"`
}

// Sandbox is returned by create and connect endpoints.
type Sandbox struct {
	TemplateID         string     `json:"templateID"`
	SandboxID          string     `json:"sandboxID"`
	Alias              string     `json:"alias,omitempty"`
	ClientID           string     `json:"clientID"`
	EnvdVersion        string     `json:"envdVersion"`
	EnvdAccessToken    *string    `json:"envdAccessToken"`
	EnvdURL            *string    `json:"envdUrl"`
	TrafficAccessToken *string    `json:"trafficAccessToken"`
	Namespace          string     `json:"namespace,omitempty"`
	Status             string     `json:"status"`
	State              string     `json:"state,omitempty"`
	StartedAt          time.Time  `json:"startedAt"`
	ActivatedAt        *time.Time `json:"activatedAt,omitempty"`
	EndAt              time.Time  `json:"endAt"`
}

// SandboxDetail is returned by GET /api/v1/sandboxes/:sandboxID.
type SandboxDetail struct {
	TemplateID      string            `json:"templateID"`
	Alias           string            `json:"alias,omitempty"`
	SandboxID       string            `json:"sandboxID"`
	ClientID        string            `json:"clientID"`
	StartedAt       time.Time         `json:"startedAt"`
	EndAt           time.Time         `json:"endAt"`
	EnvdVersion     string            `json:"envdVersion"`
	EnvdAccessToken *string           `json:"envdAccessToken"`
	EnvdURL         *string           `json:"envdUrl"`
	CPUCount        int32             `json:"cpuCount"`
	MemoryMB        int32             `json:"memoryMB"`
	DiskSizeMB      int32             `json:"diskSizeMB"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Status          string            `json:"status"`
	State           string            `json:"state,omitempty"`
	VolumeMounts    []VolumeMount     `json:"volumeMounts,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	ActivatedAt     *time.Time        `json:"activatedAt,omitempty"`
}

// ListedSandbox is returned by the list endpoint.
type ListedSandbox struct {
	TemplateID   string            `json:"templateID"`
	Alias        string            `json:"alias,omitempty"`
	SandboxID    string            `json:"sandboxID"`
	ClientID     string            `json:"clientID"`
	StartedAt    time.Time         `json:"startedAt"`
	EndAt        time.Time         `json:"endAt"`
	CPUCount     int32             `json:"cpuCount"`
	MemoryMB     int32             `json:"memoryMB"`
	DiskSizeMB   int32             `json:"diskSizeMB"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Status       string            `json:"status"`
	State        string            `json:"state,omitempty"`
	EnvdVersion  string            `json:"envdVersion"`
	VolumeMounts []VolumeMount     `json:"volumeMounts,omitempty"`
	ActivatedAt  *time.Time        `json:"activatedAt,omitempty"`
}

// ListSandboxesParams configures GET /api/v1/sandboxes.
type ListSandboxesParams struct {
	Metadata  map[string]string
	State     []string
	Limit     int
	NextToken string
}

// SandboxLogsParams configures GET /api/v1/sandboxes/:sandboxID/logs.
type SandboxLogsParams struct {
	Cursor    *int64
	Limit     *int
	Direction string
	Level     string
	Search    string
}

// SandboxLogEntry is one log record.
type SandboxLogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Message   string            `json:"message"`
	Level     string            `json:"level"`
	Fields    map[string]string `json:"fields"`
}

// SandboxLogsResponse wraps sandbox log records.
type SandboxLogsResponse struct {
	Logs []SandboxLogEntry `json:"logs"`
}

// ConnectSandboxRequest is the request body for POST /connect.
type ConnectSandboxRequest struct {
	Timeout int32 `json:"timeout"`
}

// ConnectSandboxResponse keeps both the sandbox payload and HTTP status.
type ConnectSandboxResponse struct {
	StatusCode int
	Sandbox    *Sandbox
}

// TimeoutRequest is the request body for POST /timeout.
type TimeoutRequest struct {
	Timeout int32 `json:"timeout"`
}

// RefreshSandboxRequest is the request body for POST /refreshes.
type RefreshSandboxRequest struct {
	Duration *int32 `json:"duration,omitempty"`
}

// HeartbeatRequest is the request body for POST /heartbeat.
type HeartbeatRequest struct {
	Status string `json:"status"`
}

// HeartbeatResponse is the wrapped success payload from POST /heartbeat.
type HeartbeatResponse struct {
	Received  bool   `json:"received"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

// PoolStatus is the wrapped success payload from GET /admin/pool/status.
type PoolStatus struct {
	Total       int     `json:"total"`
	Warm        int     `json:"warm"`
	Active      int     `json:"active"`
	Creating    int     `json:"creating"`
	Stopped     int     `json:"stopped"`
	Deleting    int     `json:"deleting"`
	Deleted     int     `json:"deleted"`
	Utilization float64 `json:"utilization"`
	RequestID   string  `json:"request_id,omitempty"`
}

// RollingStartRequest is the request body for POST /admin/rolling/start.
type RollingStartRequest struct {
	TemplateID string `json:"templateId"`
}

// RollingUpdateStatus is the wrapped success payload for admin rolling APIs.
type RollingUpdateStatus struct {
	Phase       string     `json:"phase"`
	Progress    float64    `json:"progress"`
	WarmTotal   int        `json:"warm_total"`
	WarmUpdated int        `json:"warm_updated"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    string     `json:"duration,omitempty"`
	RequestID   string     `json:"request_id,omitempty"`
}
