package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sandbox "github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/build"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

const (
	webPort             = 3000
	defaultWorkspaceDir = "/agent-workspace"
)

type webSource struct {
	name     string
	required bool
}

var webSources = []webSource{
	{name: "package.json", required: true},
	{name: "package-lock.json", required: true},
	{name: "index.html", required: true},
	{name: "src", required: true},
	{name: "public"},
	{name: "tsconfig.json"},
	{name: "tsconfig.app.json"},
	{name: "tsconfig.node.json"},
	{name: "vite.config.js"},
	{name: "vite.config.mjs"},
	{name: "vite.config.ts"},
	{name: "postcss.config.js"},
	{name: "postcss.config.cjs"},
	{name: "postcss.config.mjs"},
	{name: "tailwind.config.js"},
	{name: "tailwind.config.cjs"},
	{name: "tailwind.config.mjs"},
	{name: "tailwind.config.ts"},
	{name: "eslint.config.js"},
	{name: "eslint.config.mjs"},
	{name: "eslint.config.ts"},
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	baseURL, err := requiredEnv("SEAINFRA_BASE_URL")
	if err != nil {
		return err
	}
	apiKey, err := requiredEnv("SEAINFRA_API_KEY")
	if err != nil {
		return err
	}
	sourceDir, err := requiredEnv("SANDBOX_EXAMPLE_WEB_SOURCE_DIR")
	if err != nil {
		return err
	}
	workspaceDir := envOrDefault("SANDBOX_EXAMPLE_NFS_WORKSPACE_DIR", defaultWorkspaceDir)
	if !filepath.IsAbs(workspaceDir) || filepath.Clean(workspaceDir) == "/" {
		return fmt.Errorf("SANDBOX_EXAMPLE_NFS_WORKSPACE_DIR must be an absolute non-root path")
	}
	nfsHostPath, err := requiredEnv("SANDBOX_EXAMPLE_NFS_HOST_PATH")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(nfsHostPath) || filepath.Clean(nfsHostPath) == "/" {
		return fmt.Errorf("SANDBOX_EXAMPLE_NFS_HOST_PATH must be an absolute non-root path")
	}
	baseTemplateRef := envOrDefault("SANDBOX_EXAMPLE_NFS_BASE_TEMPLATE", "nfs")
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")

	client, err := sandbox.NewClient(baseURL, apiKey, core.WithTimeout(4*time.Minute))
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	baseTemplate, err := client.Build.ResolveTemplateRef(ctx, baseTemplateRef)
	if err != nil {
		return fmt.Errorf("resolve NFS base template %q: %w", baseTemplateRef, err)
	}
	if baseTemplate == nil || strings.TrimSpace(baseTemplate.TemplateID) == "" {
		return fmt.Errorf("resolve NFS base template %q: empty template ID", baseTemplateRef)
	}
	log.Printf("resolved NFS base template: ref=%s template=%s", baseTemplateRef, baseTemplate.TemplateID)

	template := sandbox.NewTemplate().
		FromTemplate(baseTemplate.TemplateID).
		SetWorkdir("/app")
	if err := addWebSources(template, sourceDir); err != nil {
		return err
	}
	template.
		RunCmd("apk add --no-cache nodejs npm curl", nil).
		RunCmd("npm ci --no-audit --no-fund", nil).
		RunCmd("npm run build", nil).
		SetStartCmd(buildWebStartCommand(workspaceDir), sandbox.WaitForURL("http://127.0.0.1:3000/", http.StatusOK))

	templateName := envOrDefault(
		"SANDBOX_EXAMPLE_TEMPLATE_NAME",
		"go-nfs-web-example-"+strconv.FormatInt(time.Now().UTC().UnixNano(), 10),
	)
	built, buildErr := client.BuildTemplate(ctx, template, templateName, &sandbox.TemplateBuildOptions{
		BaseTemplateID: baseTemplate.TemplateID,
		Visibility:     "personal",
		Workdir:        workspaceDir,
		VolumeMounts: []build.TemplateVolumeMount{{
			Name:        "workspace",
			Path:        workspaceDir,
			StorageType: "nfs",
			NFSHostPath: nfsHostPath,
		}},
		WaitTimeout:  30 * time.Minute,
		PollInterval: 2 * time.Second,
		OnBuildLog: func(entry sandbox.LogEntry) {
			log.Println(entry.String())
		},
	})
	if built != nil && built.TemplateID != "" && !keepResources {
		defer deleteTemplate(client, built.TemplateID)
	}
	if buildErr != nil {
		return fmt.Errorf("build NFS Web template: %w", buildErr)
	}
	if built == nil || built.TemplateID == "" {
		return fmt.Errorf("build NFS Web template: empty result")
	}
	log.Printf("template ready: template=%s build=%s status=%s", built.TemplateID, built.BuildID, built.Status)

	waitReady := true
	autoPause := false
	timeoutSeconds := int32(900)
	created, createErr := client.Create(ctx, built.TemplateID, &sandbox.CreateOptions{
		WaitReady:    &waitReady,
		AutoPause:    &autoPause,
		Timeout:      &timeoutSeconds,
		WaitTimeout:  8 * time.Minute,
		PollInterval: 2 * time.Second,
		Metadata:     map[string]string{"source": "sandbox-go-nfs-web-example"},
	})
	if created != nil && created.SandboxID != "" && !keepResources {
		defer deleteSandbox(created)
	}
	if createErr != nil {
		return fmt.Errorf("create sandbox from NFS Web template: %w", createErr)
	}
	if created == nil || created.SandboxID == "" {
		return fmt.Errorf("create sandbox from NFS Web template: empty result")
	}
	log.Printf("sandbox ready: sandbox=%s status=%s", created.SandboxID, created.Status)

	if err := verifyWorkspaceMount(ctx, created, workspaceDir); err != nil {
		return err
	}
	if err := verifyRuntime(ctx, created, workspaceDir); err != nil {
		return err
	}
	markerPath := filepath.ToSlash(filepath.Join(workspaceDir, ".sandbox-go-nfs-marker"))
	markerValue := "nfs-persisted-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	if err := writeAndVerifyMarker(ctx, created, markerPath, markerValue); err != nil {
		return err
	}

	if err := created.Pause(ctx); err != nil {
		return fmt.Errorf("pause sandbox: %w", err)
	}
	pauseCtx, cancelPause := context.WithTimeout(ctx, 3*time.Minute)
	_, err = waitForPaused(pauseCtx, client, created.SandboxID)
	cancelPause()
	if err != nil {
		return err
	}
	log.Printf("sandbox paused: sandbox=%s", created.SandboxID)

	resumed, err := created.Resume(ctx, 600)
	if err != nil {
		return fmt.Errorf("resume sandbox: %w", err)
	}
	if resumed == nil || resumed.SandboxID != created.SandboxID {
		return fmt.Errorf("resume sandbox: unexpected result %#v", resumed)
	}
	resumeCtx, cancelResume := context.WithTimeout(ctx, 5*time.Minute)
	if err := waitForRuntime(resumeCtx, resumed, markerPath, markerValue); err != nil {
		cancelResume()
		return err
	}
	if err := waitForWeb(resumeCtx, resumed); err != nil {
		cancelResume()
		return err
	}
	cancelResume()
	log.Printf("NFS workspace survived pause/resume: sandbox=%s marker=%s", resumed.SandboxID, markerPath)

	if host, err := resumed.GetHost(webPort); err != nil {
		return fmt.Errorf("resolve Web proxy URL: %w", err)
	} else {
		log.Printf("Web proxy URL: %s", host)
	}
	if keepResources {
		log.Printf("kept resources: template=%s sandbox=%s", built.TemplateID, resumed.SandboxID)
	}
	return nil
}

func addWebSources(template *sandbox.Template, sourceDir string) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("inspect Web source directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("SANDBOX_EXAMPLE_WEB_SOURCE_DIR must be a directory")
	}

	for _, source := range webSources {
		localPath := filepath.Join(sourceDir, source.name)
		if _, err := os.Stat(localPath); err != nil {
			if os.IsNotExist(err) && !source.required {
				continue
			}
			return fmt.Errorf("required Web source %s: %w", source.name, err)
		}
		template.Copy(localPath, filepath.ToSlash(filepath.Join("/app", source.name)), nil)
	}
	return nil
}

func buildWebStartCommand(workspaceDir string) string {
	quotedWorkspace := shellQuote(workspaceDir)
	return fmt.Sprintf(`set -eu
workspace=%s
mkdir -p "$workspace"
if [ -z "$(ls -A "$workspace" 2>/dev/null)" ]; then
  cp -a /app/. "$workspace"/
fi
cd "$workspace"
exec npm run dev -- --host 0.0.0.0 --port 3000`, quotedWorkspace)
}

func verifyWorkspaceMount(ctx context.Context, created *sandbox.Sandbox, workspaceDir string) error {
	detail, err := created.Reload(ctx)
	if err != nil {
		return fmt.Errorf("load sandbox mounts: %w", err)
	}
	for _, mount := range detail.VolumeMounts {
		if mount.Name == "workspace" && filepath.Clean(mount.Path) == filepath.Clean(workspaceDir) {
			log.Printf("derived NFS workspace override verified: name=%s path=%s", mount.Name, mount.Path)
			return nil
		}
	}
	return fmt.Errorf(
		"official NFS template does not mount workspace at %s; set SANDBOX_EXAMPLE_NFS_WORKSPACE_DIR to the reported workspace mount",
		workspaceDir,
	)
}

func verifyRuntime(ctx context.Context, created *sandbox.Sandbox, workspaceDir string) error {
	commands, err := created.Commands()
	if err != nil {
		return fmt.Errorf("bind executor commands: %w", err)
	}
	result, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "test -f package.json && test -d src && pwd"},
		CWD:  workspaceDir,
	})
	if err != nil {
		return fmt.Errorf("verify executor and initialized workspace: %w", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != workspaceDir {
		return fmt.Errorf(
			"verify initialized workspace: exit=%d stdout=%q stderr=%q",
			result.ExitCode,
			result.Stdout,
			result.Stderr,
		)
	}
	if err := verifyWeb(ctx, created); err != nil {
		return fmt.Errorf("verify Web service: %w", err)
	}
	log.Printf("executor and Web service verified: executor_port=9000 web_port=%d", webPort)
	return nil
}

func verifyWeb(ctx context.Context, created *sandbox.Sandbox) error {
	response, err := created.Proxy(ctx, &cmd.ProxyRequest{Method: http.MethodGet, Port: webPort})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("Web service returned an empty response")
	}
	return nil
}

func waitForWeb(ctx context.Context, created *sandbox.Sandbox) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := verifyWeb(ctx, created); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Web service after resume: %w (last error: %v)", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func writeAndVerifyMarker(ctx context.Context, created *sandbox.Sandbox, path, value string) error {
	files, err := created.Files()
	if err != nil {
		return fmt.Errorf("bind executor files: %w", err)
	}
	if _, err := files.Write(ctx, path, []byte(value)); err != nil {
		return fmt.Errorf("write NFS marker: %w", err)
	}
	body, err := files.Read(ctx, path)
	if err != nil {
		return fmt.Errorf("read NFS marker: %w", err)
	}
	if strings.TrimSpace(body) != value {
		return fmt.Errorf("verify NFS marker: got %q, want %q", body, value)
	}
	return nil
}

func waitForRuntime(ctx context.Context, created *sandbox.Sandbox, markerPath, markerValue string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		files, err := created.Files()
		if err == nil {
			body, readErr := files.Read(ctx, markerPath)
			if readErr == nil && strings.TrimSpace(body) == markerValue {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for resumed executor and NFS marker: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForPaused(ctx context.Context, client *sandbox.Client, sandboxID string) (*sandbox.SandboxDetail, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		detail, err := client.Get(ctx, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("get sandbox while waiting for pause: %w", err)
		}
		state := strings.ToLower(strings.TrimSpace(detail.State))
		status := strings.ToLower(strings.TrimSpace(detail.Status))
		if state == "paused" || state == "stopped" || status == "paused" || status == "stopped" {
			return detail, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for paused sandbox: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func deleteSandbox(created *sandbox.Sandbox) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := created.Delete(ctx); err != nil {
		log.Printf("delete sandbox warning: %v", err)
		return
	}
	log.Printf("deleted sandbox=%s", created.SandboxID)
}

func deleteTemplate(client *sandbox.Client, templateID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := client.DeleteTemplate(ctx, templateID); err != nil {
		log.Printf("delete template warning: %v", err)
		return
	}
	log.Printf("deleted template=%s", templateID)
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
