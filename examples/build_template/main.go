package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

const defaultBuildContext = "examples/build_template/context"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()

	baseURL := strings.TrimSpace(os.Getenv("SEAINFRA_BASE_URL"))
	if baseURL == "" {
		return fmt.Errorf("SEAINFRA_BASE_URL is required")
	}
	apiKey := strings.TrimSpace(os.Getenv("SEAINFRA_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("SEAINFRA_API_KEY is required")
	}

	baseTemplateRef := envOrDefault("SANDBOX_EXAMPLE_BASE_TEMPLATE", "node")
	buildContext := envOrDefault("SANDBOX_EXAMPLE_BUILD_CONTEXT", defaultBuildContext)
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")

	client, err := sandbox.NewClient(baseURL, apiKey, core.WithTimeout(4*time.Minute))
	if err != nil {
		return err
	}
	baseTemplate, err := client.Build.ResolveTemplateRef(ctx, baseTemplateRef)
	if err != nil {
		return fmt.Errorf("resolve base template %q: %w", baseTemplateRef, err)
	}
	if baseTemplate == nil || strings.TrimSpace(baseTemplate.TemplateID) == "" {
		return fmt.Errorf("resolve base template %q: empty template ID", baseTemplateRef)
	}
	log.Printf("resolved base template: ref=%s template=%s", baseTemplateRef, baseTemplate.TemplateID)

	name := "go-build-example-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	built, err := client.BuildTemplate(
		ctx,
		sandbox.NewTemplate().
			FromTemplate(baseTemplate.TemplateID).
			SetWorkdir("/app").
			Copy(buildContext, "/app", nil).
			RunCmd("test -f /app/sandbox-go-build-context.txt", nil),
		name,
		&sandbox.TemplateBuildOptions{
			BaseTemplateID: baseTemplate.TemplateID,
			Visibility:     "personal",
			Workdir:        "/app",
			WaitTimeout:    30 * time.Minute,
			PollInterval:   2 * time.Second,
			OnBuildLog: func(entry sandbox.LogEntry) {
				log.Println(entry.String())
			},
		},
	)
	if built != nil && built.TemplateID != "" && !keepResources {
		defer deleteTemplate(client, built.TemplateID)
	}
	if err != nil {
		return fmt.Errorf("build template: %w", err)
	}
	if built == nil || built.TemplateID == "" {
		return fmt.Errorf("build template: empty result")
	}
	log.Printf("template ready: template=%s build=%s status=%s", built.TemplateID, built.BuildID, built.Status)

	waitReady := true
	autoPause := false
	timeout := int32(600)
	created, err := client.Create(ctx, built.TemplateID, &sandbox.CreateOptions{
		WaitReady:    &waitReady,
		AutoPause:    &autoPause,
		Timeout:      &timeout,
		WaitTimeout:  8 * time.Minute,
		PollInterval: 2 * time.Second,
		Metadata:     map[string]string{"source": "sandbox-go-build-example"},
	})
	if created != nil && created.SandboxID != "" && !keepResources {
		defer deleteSandbox(created)
	}
	if err != nil {
		return fmt.Errorf("create sandbox from built template: %w", err)
	}
	log.Printf("sandbox ready: sandbox=%s status=%s", created.SandboxID, created.Status)

	commands, err := created.Commands()
	if err != nil {
		return fmt.Errorf("bind sandbox commands: %w", err)
	}
	result, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "cat /app/sandbox-go-build-context.txt"},
		CWD:  "/app",
	})
	if err != nil {
		return fmt.Errorf("verify copied build context: %w", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "sandbox-go-copy-ok" {
		return fmt.Errorf("verify copied build context: exit=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	log.Printf("copy verified: sandbox=%s output=%q", created.SandboxID, strings.TrimSpace(result.Stdout))

	if keepResources {
		log.Printf("kept resources: template=%s sandbox=%s", built.TemplateID, created.SandboxID)
	}
	return nil
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
