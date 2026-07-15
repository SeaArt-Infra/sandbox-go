package build

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	buildDirectionForward  = "forward"
	buildDirectionBackward = "backward"
	buildSourceTemporary   = "temporary"
	buildSourcePersistent  = "persistent"
	maxBuildLogsLimit      = 100
	maxBuildStatusLimit    = 100
	maxTemplateListLimit   = 100
	maxTemplateBuildLimit  = 100
)

var (
	dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	sha256Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

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

func (c *Service) DirectBuild(ctx context.Context, req *DirectBuildRequest) (*DirectBuildResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sandbox: direct build request is required")
	}

	var resp DirectBuildResponse
	if _, err := c.DoJSON(ctx, http.MethodPost, "/build", nil, nil, req, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) CreateTemplate(ctx context.Context, req *TemplateCreateRequest) (*TemplateCreateResponse, error) {
	if err := validateTemplateCreateRequest(req); err != nil {
		return nil, err
	}

	var resp TemplateCreateResponse
	if _, err := c.DoJSON(ctx, http.MethodPost, "/api/v1/templates", nil, nil, req, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListTemplates(ctx context.Context, params *ListTemplatesParams) ([]ListedTemplate, error) {
	if err := validateListTemplatesParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp []ListedTemplate
	if _, err := c.DoJSON(ctx, http.MethodGet, "/api/v1/templates", nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) GetTemplateByAlias(ctx context.Context, alias string) (*TemplateAliasResponse, error) {
	if strings.TrimSpace(alias) == "" {
		return nil, ErrAliasEmpty
	}

	var resp TemplateAliasResponse
	path := "/api/v1/templates/aliases/" + url.PathEscape(alias)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ResolveTemplateRef(ctx context.Context, ref string) (*TemplateAliasResponse, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("sandbox: ref is required")
	}

	var resp TemplateAliasResponse
	path := "/api/v1/templates/resolve/" + url.PathEscape(ref)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetTemplate(ctx context.Context, templateID string, params *GetTemplateParams) (*TemplateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if err := validateGetTemplateParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp TemplateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) UpdateTemplate(ctx context.Context, templateID string, req *TemplateUpdateRequest) (*TemplateUpdateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if err := validateTemplateUpdateRequest(req); err != nil {
		return nil, err
	}

	var resp TemplateUpdateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID)
	if _, err := c.DoJSON(ctx, http.MethodPatch, path, nil, nil, req, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) DeleteTemplate(ctx context.Context, templateID string) error {
	if strings.TrimSpace(templateID) == "" {
		return ErrTemplateEmpty
	}

	path := "/api/v1/templates/" + url.PathEscape(templateID)
	_, err := c.DoRequest(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent)
	return err
}

func (c *Service) CreateBuild(ctx context.Context, templateID, buildID string, req *BuildRequest) (*BuildTriggerResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}
	if len(strings.TrimSpace(buildID)) > 63 || !dnsLabelPattern.MatchString(strings.TrimSpace(buildID)) {
		return nil, fmt.Errorf("sandbox: buildID must be a lowercase DNS label up to 63 characters")
	}
	if err := validateBuildRequest(req); err != nil {
		return nil, err
	}

	var body any
	if !isZeroBuildRequest(req) {
		body = req
	}

	var resp BuildTriggerResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID)
	if _, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, body, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuildFile(ctx context.Context, templateID, hash string) (*FilePresenceResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(hash) == "" {
		return nil, ErrHashEmpty
	}
	if !sha256Pattern.MatchString(hash) {
		return nil, fmt.Errorf("sandbox: hash must be a 64-character lowercase hex SHA256")
	}

	var resp FilePresenceResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/files/" + url.PathEscape(hash)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) RollbackTemplate(ctx context.Context, templateID string, req *RollbackRequest) (*TemplateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if req == nil || strings.TrimSpace(req.BuildID) == "" {
		return nil, ErrBuildEmpty
	}

	var resp TemplateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/rollback"
	if _, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, req, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListBuilds(ctx context.Context, templateID string) (*BuildHistoryResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}

	var resp BuildHistoryResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuild(ctx context.Context, templateID, buildID string) (*BuildResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}

	var resp BuildResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuildStatus(ctx context.Context, templateID, buildID string, params *BuildStatusParams) (*BuildStatusResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}
	if err := validateBuildStatusParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp BuildStatusResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID) + "/status"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuildLogs(ctx context.Context, templateID, buildID string, params *BuildLogsParams) (*BuildLogsResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}
	if err := validateBuildLogsParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp BuildLogsResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID) + "/logs"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *ListTemplatesParams) encode() url.Values {
	values := make(url.Values)
	if visibility := strings.TrimSpace(p.Visibility); visibility != "" {
		values.Set("visibility", visibility)
	}
	if teamID := strings.TrimSpace(p.TeamID); teamID != "" {
		values.Set("teamID", teamID)
	}
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		values.Set("offset", strconv.Itoa(p.Offset))
	}
	return values
}

func (p *GetTemplateParams) encode() url.Values {
	values := make(url.Values)
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if nextToken := strings.TrimSpace(p.NextToken); nextToken != "" {
		values.Set("nextToken", nextToken)
	}
	return values
}

func (p *BuildStatusParams) encode() url.Values {
	values := make(url.Values)
	if p.LogsOffset != nil {
		values.Set("logsOffset", strconv.Itoa(*p.LogsOffset))
	}
	if p.Limit != nil {
		values.Set("limit", strconv.Itoa(*p.Limit))
	}
	if level := strings.TrimSpace(p.Level); level != "" {
		values.Set("level", level)
	}
	return values
}

func (p *BuildLogsParams) encode() url.Values {
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
	if source := strings.TrimSpace(p.Source); source != "" {
		values.Set("source", source)
	}
	return values
}

func validateListTemplatesParams(params *ListTemplatesParams) error {
	if params == nil {
		return nil
	}
	if params.Limit < 0 || params.Limit > maxTemplateListLimit {
		return fmt.Errorf("sandbox: template list limit must be between 0 and %d", maxTemplateListLimit)
	}
	if params.Offset < 0 {
		return fmt.Errorf("sandbox: template list offset must be non-negative")
	}
	return nil
}

func validateGetTemplateParams(params *GetTemplateParams) error {
	if params == nil {
		return nil
	}
	if params.Limit < 0 || params.Limit > maxTemplateBuildLimit {
		return fmt.Errorf("sandbox: template build history limit must be between 0 and %d", maxTemplateBuildLimit)
	}
	return nil
}

func validateTemplateCreateRequest(req *TemplateCreateRequest) error {
	if req == nil {
		return nil
	}
	return validatePublicTemplateExtensions(req.Extensions)
}

func validateTemplateUpdateRequest(req *TemplateUpdateRequest) error {
	if req == nil {
		return nil
	}
	return validatePublicTemplateExtensions(req.Extensions)
}

func validatePublicTemplateExtensions(ext *PublicTemplateExtensions) error {
	if ext == nil || ext.Seacloud == nil {
		return nil
	}
	seacloud := ext.Seacloud
	if strings.TrimSpace(seacloud.Visibility) == "official" {
		return fmt.Errorf("sandbox: extensions.seacloud.visibility=official is not supported by the public SDK")
	}
	return nil
}

func validateBuildRequest(req *BuildRequest) error {
	if req == nil {
		return nil
	}
	if req.FromImageRegistry != nil {
		if err := validateRegistryConfig(req.FromImageRegistry); err != nil {
			return err
		}
	}
	hash := strings.TrimSpace(req.FilesHash)
	if hash != "" {
		if !sha256Pattern.MatchString(hash) {
			return fmt.Errorf("sandbox: filesHash must be a 64-character lowercase hex SHA256")
		}
	}

	for i, step := range req.Steps {
		stepType := strings.ToUpper(strings.TrimSpace(step.Type))
		switch stepType {
		case "":
			return fmt.Errorf("sandbox: steps[%d].type is required", i)
		case "COPY":
			hash := strings.TrimSpace(step.FilesHash)
			if hash == "" {
				return fmt.Errorf("sandbox: steps[%d].filesHash is required for COPY", i)
			}
			if !sha256Pattern.MatchString(hash) {
				return fmt.Errorf("sandbox: steps[%d].filesHash must be a 64-character lowercase hex SHA256", i)
			}
			if len(step.Args) < 2 {
				return fmt.Errorf("sandbox: steps[%d].args must include src and dest for COPY", i)
			}
		case "ENV":
			if len(step.Args) == 0 || len(step.Args)%2 != 0 {
				return fmt.Errorf("sandbox: steps[%d].args must contain ENV key/value pairs", i)
			}
		case "RUN", "WORKDIR", "USER":
			if len(step.Args) == 0 || strings.TrimSpace(step.Args[0]) == "" {
				return fmt.Errorf("sandbox: steps[%d].args must include the %s value", i, stepType)
			}
		default:
			return fmt.Errorf("sandbox: steps[%d].type must be one of COPY, ENV, RUN, WORKDIR, USER", i)
		}
	}
	return nil
}

func validateRegistryConfig(config map[string]any) error {
	typeValue, _ := config["type"].(string)
	typeValue = strings.TrimSpace(typeValue)
	if typeValue == "" {
		return fmt.Errorf("sandbox: fromImageRegistry.type is required")
	}
	switch typeValue {
	case "registry":
		if strings.TrimSpace(asString(config["username"])) == "" || strings.TrimSpace(asString(config["password"])) == "" {
			return fmt.Errorf("sandbox: fromImageRegistry registry config requires username and password")
		}
	case "aws":
		if strings.TrimSpace(asString(config["awsAccessKeyId"])) == "" || strings.TrimSpace(asString(config["awsSecretAccessKey"])) == "" || strings.TrimSpace(asString(config["awsRegion"])) == "" {
			return fmt.Errorf("sandbox: fromImageRegistry aws config requires awsAccessKeyId, awsSecretAccessKey, and awsRegion")
		}
	case "gcp":
		if strings.TrimSpace(asString(config["serviceAccountJson"])) == "" {
			return fmt.Errorf("sandbox: fromImageRegistry gcp config requires serviceAccountJson")
		}
	default:
		return fmt.Errorf("sandbox: fromImageRegistry.type %q is not supported", typeValue)
	}
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func validateBuildStatusParams(params *BuildStatusParams) error {
	if params == nil {
		return nil
	}
	if params.LogsOffset != nil && *params.LogsOffset < 0 {
		return fmt.Errorf("sandbox: build logsOffset must be non-negative")
	}
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > maxBuildStatusLimit) {
		return fmt.Errorf("sandbox: build status limit must be between 0 and %d", maxBuildStatusLimit)
	}
	return nil
}

func validateBuildLogsParams(params *BuildLogsParams) error {
	if params == nil {
		return nil
	}
	if params.Cursor != nil && *params.Cursor < 0 {
		return fmt.Errorf("sandbox: build logs cursor must be non-negative")
	}
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > maxBuildLogsLimit) {
		return fmt.Errorf("sandbox: build logs limit must be between 0 and %d", maxBuildLogsLimit)
	}
	if direction := strings.TrimSpace(params.Direction); direction != "" &&
		direction != buildDirectionForward && direction != buildDirectionBackward {
		return fmt.Errorf("sandbox: build logs direction must be %q or %q", buildDirectionForward, buildDirectionBackward)
	}
	if source := strings.TrimSpace(params.Source); source != "" &&
		source != buildSourceTemporary && source != buildSourcePersistent {
		return fmt.Errorf("sandbox: build logs source must be %q or %q", buildSourceTemporary, buildSourcePersistent)
	}
	return nil
}

func isZeroBuildRequest(req *BuildRequest) bool {
	if req == nil {
		return true
	}
	return strings.TrimSpace(req.FromTemplate) == "" &&
		strings.TrimSpace(req.FromImage) == "" &&
		req.FromImageRegistry == nil &&
		req.Force == nil &&
		len(req.Steps) == 0 &&
		strings.TrimSpace(req.FilesHash) == "" &&
		strings.TrimSpace(req.StartCmd) == "" &&
		strings.TrimSpace(req.ReadyCmd) == ""
}
