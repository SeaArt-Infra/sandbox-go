package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/build"
)

var terminalBuildStatuses = map[string]bool{
	"ready":     true,
	"failed":    true,
	"error":     true,
	"cancelled": true,
}

func main() {
	ctx := context.Background()
	baseURL := mustEnv("SEAINFRA_BASE_URL")
	apiKey := mustEnv("SEAINFRA_API_KEY")
	image := strings.TrimSpace(os.Getenv("SANDBOX_EXAMPLE_BUILD_IMAGE"))
	if image == "" {
		image = "docker.io/library/alpine:3.20"
	}
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")
	templateName := fmt.Sprintf("go-template-features-%d:v1", time.Now().UnixNano())

	tempRoot, err := os.MkdirTemp("", "sandbox-go-template-features-")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempRoot); err != nil {
			log.Printf("remove temp dir warning: %v", err)
		}
	}()

	dockerfilePath, linkedFile, err := prepareDockerfileFixture(tempRoot, image)
	if err != nil {
		log.Fatal(err)
	}

	template, err := sandbox.NewTemplate().FromDockerfile(dockerfilePath)
	if err != nil {
		log.Fatal(err)
	}
	mode := 0o600
	template.
		SkipCache().
		RunCmd("printf 'extra build step from go template features\\n' >/workspace/extra-step.txt", &sandbox.TemplateCommandOptions{User: "root"}).
		Copy(linkedFile, "/workspace/copied-link.txt", &sandbox.TemplateCopyOptions{
			Mode:            &mode,
			ResolveSymlinks: true,
		})

	requestJSON, err := sandbox.TemplateToJSON(template, true)
	if err != nil {
		log.Fatal(err)
	}
	var request map[string]any
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		log.Fatal(err)
	}
	steps, _ := request["steps"].([]any)
	log.Printf("template request: from=%v steps=%d start=%v", request["fromImage"], len(steps), request["startCmd"])

	dockerfileText, err := sandbox.TemplateToDockerfile(template)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("dockerfile preview: %s", dockerfilePreview(dockerfileText))

	client, err := sandbox.NewClient(baseURL, apiKey)
	if err != nil {
		log.Fatal(err)
	}

	built, err := client.BuildTemplateInBackground(ctx, template, templateName, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build started: template=%s build=%s status=%s", built.TemplateID, built.BuildID, built.Status)

	templateID := built.TemplateID
	if !keepResources {
		defer func() {
			if err := client.DeleteTemplate(ctx, templateID); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", templateID)
		}()
	}

	status, err := waitForBuild(ctx, client, built.TemplateID, built.BuildID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build finished: status=%s last=%q", status.Status, latestBuildLog(status))
	if status.Status != "ready" {
		log.Fatalf("template build did not succeed: %s", status.Status)
	}

	exists, err := client.TemplateExists(ctx, built.TemplateID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("template exists: %t", exists)

	detail, err := client.GetTemplate(ctx, built.TemplateID, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("template detail: template=%s status=%s names=%v", detail.TemplateID, detail.BuildStatus, detail.Names)

	if keepResources {
		log.Printf("kept template=%s", built.TemplateID)
	}
}

func prepareDockerfileFixture(root, image string) (dockerfilePath string, linkedFile string, err error) {
	source := filepath.Join(root, "artifact.txt")
	linkedFile = filepath.Join(root, "artifact-link.txt")
	dockerfilePath = filepath.Join(root, "Dockerfile")

	if err := os.WriteFile(source, []byte("hello from go template features\n"), 0o644); err != nil {
		return "", "", err
	}
	if err := os.Symlink(source, linkedFile); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(dockerfilePath, []byte(strings.Join([]string{
		"FROM " + image,
		"WORKDIR /workspace",
		"COPY ./artifact.txt /workspace/from-dockerfile.txt",
		`CMD ["sleep", "infinity"]`,
		"",
	}, "\n")), 0o644); err != nil {
		return "", "", err
	}
	return dockerfilePath, linkedFile, nil
}

func waitForBuild(ctx context.Context, client *sandbox.Client, templateID, buildID string) (*build.BuildStatusResponse, error) {
	logsOffset := 0
	for {
		limit := 100
		status, err := client.GetTemplateBuildStatus(ctx, templateID, buildID, &sandbox.TemplateBuildStatusOptions{
			LogsOffset: &logsOffset,
			Limit:      &limit,
		})
		if err != nil {
			return nil, err
		}

		for _, entry := range status.LogEntries {
			log.Printf("build log: %s %s %s", entry.Level, entry.Step, entry.Message)
		}
		logsOffset += len(status.LogEntries)

		if terminalBuildStatuses[status.Status] {
			return status, nil
		}

		time.Sleep(2 * time.Second)
	}
}

func latestBuildLog(status *build.BuildStatusResponse) string {
	if status != nil && len(status.LogEntries) > 0 {
		return status.LogEntries[len(status.LogEntries)-1].Message
	}
	if status != nil && len(status.Logs) > 0 {
		return status.Logs[len(status.Logs)-1]
	}
	return ""
}

func dockerfilePreview(dockerfile string) string {
	lines := strings.Split(dockerfile, "\n")
	if len(lines) > 4 {
		lines = lines[:4]
	}
	return strings.Join(lines, " | ")
}

func mustEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
