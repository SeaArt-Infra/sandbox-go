package build

import "time"

// DirectBuildRequest is the request body for POST /build.
type DirectBuildRequest struct {
	Project    string `json:"project"`
	Image      string `json:"image"`
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
}

// DirectBuildResponse is returned by POST /build.
type DirectBuildResponse struct {
	TemplateID    string `json:"templateID"`
	BuildID       string `json:"buildID"`
	ImageFullName string `json:"imageFullName"`
}

type PublicSeacloudTemplateExtensions struct {
	BaseTemplateID string            `json:"baseTemplateID,omitempty"`
	Visibility     string            `json:"visibility,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	StorageType    string            `json:"storageType,omitempty"`
	StorageSizeGB  *int32            `json:"storageSizeGB,omitempty"`
}

type PublicTemplateExtensions struct {
	Seacloud *PublicSeacloudTemplateExtensions `json:"seacloud,omitempty"`
}

type SeacloudTemplateExtensions struct {
	BaseTemplateID string            `json:"baseTemplateID,omitempty"`
	Visibility     string            `json:"visibility,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	StorageType    string            `json:"storageType,omitempty"`
	StorageSizeGB  *int32            `json:"storageSizeGB,omitempty"`
	Image          string            `json:"image,omitempty"`
	ImageSource    string            `json:"imageSource,omitempty"`
	ProjectID      string            `json:"projectID,omitempty"`
	TTLSeconds     *int32            `json:"ttlSeconds,omitempty"`
	Port           *int32            `json:"port,omitempty"`
	StartCmd       string            `json:"startCmd,omitempty"`
	ReadyCmd       string            `json:"readyCmd,omitempty"`
}

type TemplateExtensions struct {
	Seacloud *SeacloudTemplateExtensions `json:"seacloud,omitempty"`
}

// TemplateCreateRequest is the request body for POST /api/v1/templates.
type TemplateCreateRequest struct {
	Name       string                    `json:"name,omitempty"`
	Tags       []string                  `json:"tags,omitempty"`
	CPUCount   *int32                    `json:"cpuCount,omitempty"`
	MemoryMB   *int32                    `json:"memoryMB,omitempty"`
	Extensions *PublicTemplateExtensions `json:"extensions,omitempty"`
}

// TemplateUpdateRequest is the request body for PATCH /api/v1/templates/:id.
type TemplateUpdateRequest struct {
	Extensions *PublicTemplateExtensions `json:"extensions,omitempty"`
}

// TemplateCreateResponse is the minimal create response.
type TemplateCreateResponse struct {
	TemplateID string   `json:"templateID"`
	BuildID    string   `json:"buildID"`
	Public     bool     `json:"public"`
	Names      []string `json:"names"`
	Tags       []string `json:"tags"`
	Aliases    []string `json:"aliases"`
}

// TemplateUpdateResponse is the minimal update response.
type TemplateUpdateResponse struct {
	Names []string `json:"names"`
}

// ListTemplatesParams configures GET /api/v1/templates.
type ListTemplatesParams struct {
	Visibility string
	TeamID     string
	Limit      int
	Offset     int
}

// GetTemplateParams configures GET /api/v1/templates/:id.
type GetTemplateParams struct {
	Limit     int
	NextToken string
}

// TemplateAliasResponse is the minimal alias lookup response.
type TemplateAliasResponse struct {
	TemplateID string `json:"templateID"`
	Public     bool   `json:"public"`
}

// TemplateUser describes the creator of a template.
type TemplateUser struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
}

type TemplateToleration struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect"`
}

type TemplateDNSOption struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// ListedTemplate is one item returned by GET /api/v1/templates.
type ListedTemplate struct {
	TemplateID    string              `json:"templateID"`
	BuildID       string              `json:"buildID,omitempty"`
	CPUCount      int32               `json:"cpuCount"`
	MemoryMB      int32               `json:"memoryMB"`
	DiskSizeMB    int32               `json:"diskSizeMB"`
	BuildStatus   string              `json:"buildStatus"`
	Public        bool                `json:"public"`
	Names         []string            `json:"names"`
	Aliases       []string            `json:"aliases"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	CreatedBy     *TemplateUser       `json:"createdBy"`
	LastSpawnedAt *time.Time          `json:"lastSpawnedAt"`
	SpawnCount    int64               `json:"spawnCount"`
	BuildCount    int32               `json:"buildCount"`
	EnvdVersion   string              `json:"envdVersion,omitempty"`
	Extensions    *TemplateExtensions `json:"extensions,omitempty"`
}

// TemplateResponse describes a template and its build history.
type TemplateResponse struct {
	TemplateID            string               `json:"templateID"`
	BuildID               string               `json:"buildID,omitempty"`
	BuildStatus           string               `json:"buildStatus,omitempty"`
	CPUCount              *int32               `json:"cpuCount,omitempty"`
	MemoryMB              *int32               `json:"memoryMB,omitempty"`
	DiskSizeMB            *int32               `json:"diskSizeMB,omitempty"`
	Public                bool                 `json:"public"`
	Aliases               []string             `json:"aliases"`
	Names                 []string             `json:"names"`
	CreatedBy             *TemplateUser        `json:"createdBy,omitempty"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
	LastSpawnedAt         *time.Time           `json:"lastSpawnedAt"`
	SpawnCount            int64                `json:"spawnCount"`
	BuildCount            *int32               `json:"buildCount,omitempty"`
	EnvdVersion           string               `json:"envdVersion,omitempty"`
	Builds                []TemplateBuild      `json:"builds,omitempty"`
	NextToken             string               `json:"nextToken,omitempty"`
	Type                  string               `json:"type,omitempty"`
	Version               string               `json:"version,omitempty"`
	Name                  string               `json:"name,omitempty"`
	Visibility            string               `json:"visibility,omitempty"`
	BaseTemplateID        string               `json:"baseTemplateID,omitempty"`
	Image                 string               `json:"image,omitempty"`
	ImageSource           string               `json:"imageSource,omitempty"`
	ProjectID             string               `json:"projectID,omitempty"`
	StorageType           string               `json:"storageType,omitempty"`
	StorageSizeGB         *int32               `json:"storageSizeGB,omitempty"`
	StorageClass          string               `json:"storageClass,omitempty"`
	EmptyDirSizeLimit     string               `json:"emptyDirSizeLimit,omitempty"`
	EmptyDirMedium        string               `json:"emptyDirMedium,omitempty"`
	ObjectBucket          string               `json:"objectBucket,omitempty"`
	ObjectKeyPrefix       string               `json:"objectKeyPrefix,omitempty"`
	WorkspaceID           string               `json:"workspaceID,omitempty"`
	NfsHostPath           string               `json:"nfsHostPath,omitempty"`
	CPULimitRatio         *float64             `json:"cpuLimitRatio,omitempty"`
	MemoryLimitRatio      *float64             `json:"memoryLimitRatio,omitempty"`
	Namespace             string               `json:"namespace,omitempty"`
	Location              string               `json:"location,omitempty"`
	EnvdBaseDomain        string               `json:"envdBaseDomain,omitempty"`
	RuntimeClassName      string               `json:"runtimeClassName,omitempty"`
	Tolerations           []TemplateToleration `json:"tolerations,omitempty"`
	ImagePullSecrets      []string             `json:"imagePullSecrets,omitempty"`
	ImagePullPolicy       string               `json:"imagePullPolicy,omitempty"`
	DNSPolicy             string               `json:"dnsPolicy,omitempty"`
	DNSNameservers        []string             `json:"dnsNameservers,omitempty"`
	DNSSearches           []string             `json:"dnsSearches,omitempty"`
	DNSOptions            []TemplateDNSOption  `json:"dnsOptions,omitempty"`
	Port                  *int32               `json:"port,omitempty"`
	Labels                map[string]string    `json:"labels,omitempty"`
	TTLSeconds            *int32               `json:"ttlSeconds,omitempty"`
	DaemonImage           string               `json:"daemonImage,omitempty"`
	CloudsinkURL          string               `json:"cloudsinkURL,omitempty"`
	SandboxRoot           string               `json:"sandboxRoot,omitempty"`
	ProbeTargets          string               `json:"probeTargets,omitempty"`
	ProbeIntervalSecs     *int32               `json:"probeIntervalSecs,omitempty"`
	ProbeTimeoutSecs      *int32               `json:"probeTimeoutSecs,omitempty"`
	GatewayURL            string               `json:"gatewayURL,omitempty"`
	HeartbeatIntervalSecs *int32               `json:"heartbeatIntervalSecs,omitempty"`
	StartCmd              string               `json:"startCmd,omitempty"`
	ReadyCmd              string               `json:"readyCmd,omitempty"`
	Extensions            *TemplateExtensions  `json:"extensions,omitempty"`
}

// TemplateBuild is the embedded build summary returned by GET /api/v1/templates/:id.
type TemplateBuild struct {
	BuildID     string     `json:"buildID"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	FinishedAt  *time.Time `json:"finishedAt"`
	CPUCount    int32      `json:"cpuCount"`
	MemoryMB    int32      `json:"memoryMB"`
	DiskSizeMB  int32      `json:"diskSizeMB"`
	EnvdVersion string     `json:"envdVersion"`
}

// BuildStep is one build step in an E2B-compatible rebuild request.
type BuildStep struct {
	Type      string   `json:"type,omitempty"`
	FilesHash string   `json:"filesHash,omitempty"`
	Args      []string `json:"args,omitempty"`
	Force     *bool    `json:"force,omitempty"`
}

// BuildRequest is the request body for POST /api/v1/templates/:id/builds/:buildId.
type BuildRequest struct {
	FromTemplate      string         `json:"fromTemplate,omitempty"`
	FromImage         string         `json:"fromImage,omitempty"`
	FromImageRegistry map[string]any `json:"fromImageRegistry,omitempty"`
	Force             *bool          `json:"force,omitempty"`
	Steps             []BuildStep    `json:"steps,omitempty"`
	FilesHash         string         `json:"filesHash,omitempty"`
	StartCmd          string         `json:"startCmd,omitempty"`
	ReadyCmd          string         `json:"readyCmd,omitempty"`
}

// BuildResponse describes one build record.
type BuildResponse struct {
	BuildID      string     `json:"buildID"`
	TemplateID   string     `json:"templateID"`
	Status       string     `json:"status"`
	Image        string     `json:"image"`
	ErrorMessage string     `json:"errorMessage"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

// BuildTriggerResponse captures the E2B empty-object build trigger response.
type BuildTriggerResponse struct{}

// FilePresenceResponse is returned by GET /api/v1/templates/:id/files/:hash.
type FilePresenceResponse struct {
	Present bool   `json:"present"`
	URL     string `json:"url,omitempty"`
}

// RollbackRequest is the request body for POST /api/v1/templates/:id/rollback.
type RollbackRequest struct {
	BuildID string `json:"buildID"`
}

// BuildHistoryResponse is returned by GET /api/v1/templates/:id/builds.
type BuildHistoryResponse struct {
	Builds []BuildResponse `json:"builds"`
	Total  int             `json:"total"`
}

// BuildStatusParams configures GET /api/v1/templates/:id/builds/:buildID/status.
type BuildStatusParams struct {
	LogsOffset *int
	Limit      *int
	Level      string
}

// BuildStatusResponse is returned by GET /status.
type BuildStatusResponse struct {
	BuildID    string          `json:"buildID"`
	TemplateID string          `json:"templateID"`
	Status     string          `json:"status"`
	Logs       []string        `json:"logs"`
	LogEntries []BuildLogEntry `json:"logEntries"`
	Reason     any             `json:"reason"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// BuildLogsParams configures GET /logs.
type BuildLogsParams struct {
	Cursor    *int64
	Limit     *int
	Direction string
	Level     string
	Source    string
}

// BuildLogEntry is one structured build log line.
type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Step      string    `json:"step"`
	Message   string    `json:"message"`
}

// BuildLogsResponse wraps structured build logs.
type BuildLogsResponse struct {
	Logs []BuildLogEntry `json:"logs"`
}
