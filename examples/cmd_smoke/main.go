package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/SeaArt-Infra/sandbox-go"
)

func main() {
	ctx := context.Background()
	baseURL := strings.TrimSpace(os.Getenv("SEAINFRA_BASE_URL"))
	if baseURL == "" {
		log.Fatal("SEAINFRA_BASE_URL is required")
	}

	apiKey := strings.TrimSpace(os.Getenv("SEAINFRA_API_KEY"))
	if apiKey == "" {
		log.Fatal("SEAINFRA_API_KEY is required")
	}

	templateID := strings.TrimSpace(os.Getenv("SANDBOX_EXAMPLE_TEMPLATE_ID"))
	if templateID == "" {
		log.Fatal("SANDBOX_EXAMPLE_TEMPLATE_ID is required")
	}

	client, err := sandbox.NewClient(baseURL, apiKey)
	if err != nil {
		log.Fatal(err)
	}

	waitReady := true
	timeout := int32(1800)
	created, err := client.Create(ctx, templateID, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if keepResources := strings.TrimSpace(strings.ToLower(os.Getenv("SANDBOX_EXAMPLE_KEEP_RESOURCES"))); keepResources == "1" || keepResources == "true" || keepResources == "yes" {
			return
		}
		if err := created.Delete(ctx); err != nil {
			log.Printf("delete sandbox warning: %v", err)
		}
	}()

	files, err := created.Files()
	if err != nil {
		log.Fatal(err)
	}
	commands, err := created.Commands()
	if err != nil {
		log.Fatal(err)
	}

	root := "/root/workspace"
	filePath := root + "/go-cmd-example.txt"

	if _, err := files.Write(ctx, filePath, []byte("hello from go example")); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote file=%s", filePath)

	body, err := files.Read(ctx, filePath)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("file content=%q", body)

	listing, err := files.List(ctx, root, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("directory entries=%d", len(listing))

	run, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "cat " + filePath},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("run exit=%d stdout=%q stderr=%q", run.ExitCode, run.Stdout, run.Stderr)
}
