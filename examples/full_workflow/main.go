package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/build"
	"github.com/SeaArt-Infra/sandbox-go/cmd"
	"github.com/SeaArt-Infra/sandbox-go/control"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func main() {
	ctx := context.Background()

	baseURL := mustEnv("SEAINFRA_BASE_URL")
	apiKey := mustEnv("SEAINFRA_API_KEY")
	runtimeBaseImage := mustEnv("SANDBOX_EXAMPLE_RUNTIME_BASE_IMAGE")
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")

	client, err := sandbox.NewClient(
		baseURL,
		apiKey,
		core.WithTimeout(180*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	logMetricLine("control", func() (string, error) {
		return client.Metrics(ctx)
	})
	logMetricLine("build", func() (string, error) {
		return client.Build.Metrics(ctx)
	})

	templateName := fmt.Sprintf("go-full-workflow-%d", time.Now().UnixNano())
	buildLogCount := 0
	built, err := client.BuildTemplate(
		ctx,
		sandbox.NewTemplate().
			FromImage(runtimeBaseImage).
			RunCmd("mkdir -p /workspace && printf 'hello from go full workflow\\n' >/workspace/built-by-template.txt", nil).
			SetReadyCmd(sandbox.WaitForFile("/workspace/built-by-template.txt")),
		templateName,
		&sandbox.TemplateBuildOptions{
			PollInterval: 2 * time.Second,
			OnBuildLog: func(entry sandbox.LogEntry) {
				buildLogCount++
				log.Println(entry.String())
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	templateID := built.TemplateID
	buildID := built.BuildID
	log.Printf("build ready: template=%s build=%s status=%s", templateID, buildID, built.Status)
	log.Printf("build detail: status=%s image=%s", built.Build.Status, built.Build.Image)

	if !keepResources {
		defer func() {
			if err := client.DeleteTemplate(ctx, templateID); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", templateID)
		}()
	}

	buildStatus, err := client.GetTemplateBuildStatus(ctx, templateID, buildID, &sandbox.TemplateBuildStatusOptions{
		Limit: intPtr(20),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build logs: count=%d last=%q", buildLogCount, latestBuildLog(buildStatus))

	templateDetail, err := client.GetTemplate(ctx, templateID, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("template detail: names=%v nextToken=%s", templateDetail.Names, templateDetail.NextToken)

	waitReady := true
	timeout := int32(1800)
	createdSandbox, err := client.Create(ctx, templateID, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox created: sandbox=%s status=%s", createdSandbox.SandboxID, createdSandbox.Status)

	if !keepResources {
		defer func() {
			if err := createdSandbox.Delete(ctx); err != nil {
				log.Printf("delete sandbox warning: %v", err)
				return
			}
			log.Printf("deleted sandbox=%s", createdSandbox.SandboxID)
		}()
	}

	sandboxDetail, err := createdSandbox.Reload(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox detail: state=%s status=%s", sandboxDetail.State, sandboxDetail.Status)

	sandboxLogsLimit := 10
	sandboxLogs, err := sandboxDetail.Logs(ctx, &control.SandboxLogsParams{Limit: &sandboxLogsLimit})
	if err != nil {
		log.Printf("sandbox logs warning: %v", err)
	} else {
		log.Printf("sandbox logs: count=%d last=%q", len(sandboxLogs.Logs), latestSandboxLog(sandboxLogs))
	}

	connected, err := sandboxDetail.Connect(ctx, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox connected: statusCode=%d sandbox=%s", connected.StatusCode, connected.Sandbox.SandboxID)

	runtime, err := connected.Sandbox.Runtime()
	if err != nil {
		log.Fatal(err)
	}

	runtimeMetrics, err := runtime.Metrics(ctx)
	if err != nil {
		log.Printf("runtime metrics warning: %v", err)
	} else {
		log.Printf(
			"runtime metrics: cpu=%.2f%% mem=%d/%d MiB disk=%d/%d",
			runtimeMetrics.CPUUsedPct,
			runtimeMetrics.MemUsedMiB,
			runtimeMetrics.MemTotalMiB,
			runtimeMetrics.DiskUsed,
			runtimeMetrics.DiskTotal,
		)
	}

	listing, err := runtime.ListDir(ctx, &cmd.ListDirRequest{Path: "/workspace"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("workspace entries=%d", len(listing.Entries))

	runResp, err := runtime.Run(ctx, &cmd.AgentRunRequest{
		Cmd:  "sh",
		Args: []string{"-lc", "cat /workspace/built-by-template.txt && echo workflow-ok"},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("run exit=%d stdout=%q stderr=%q", runResp.ExitCode, runResp.Stdout, runResp.Stderr)

	if keepResources {
		log.Printf("kept resources: template=%s sandbox=%s", templateID, createdSandbox.SandboxID)
	}
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

func firstNonEmptyLine(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func logMetricLine(name string, fn func() (string, error)) {
	value, err := fn()
	if err != nil {
		log.Printf("%s metrics warning: %v", name, err)
		return
	}
	log.Printf("%s metrics: %s", name, firstNonEmptyLine(value))
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

func latestSandboxLog(logs *control.SandboxLogsResponse) string {
	if logs == nil || len(logs.Logs) == 0 {
		return ""
	}
	return logs.Logs[len(logs.Logs)-1].Message
}

func intPtr(value int) *int {
	return &value
}
