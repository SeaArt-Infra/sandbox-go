# Sandbox Go SDK

Go SDK for Sandbox control-plane, build-plane, and nano-executor CMD APIs.

## Install

```bash
go get github.com/SeaArt-Infra/sandbox-go
```

## Entrypoints

Preferred public API:

- initialize once: `sandbox.NewClient(baseURL, apiKey, opts...)`
- sandbox lifecycle through the root client: `client.Create`, `client.Connect`, `client.List`
- sandbox runtime modules from the returned object: `created.Commands()`, `created.Files()`, `created.Git()`, `created.Pty()`
- template builder through the root client: `sandbox.NewTemplate()` plus `client.BuildTemplate(...)`
- low-level build plane via `client.Build`
- raw runtime helper: `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(createdSandbox)`

`control` and `build` both talk to the same gateway. Runtime access is derived from sandbox create/detail/connect responses; callers should not hardcode runtime endpoints or tokens. Project-scoped deployments can inject gateway routing context with `core.WithProjectID(...)`.

## E2B Alignment

- Supported alignment target: sandbox lifecycle, files, commands, git, PTY, and the high-level template DSL are designed to follow the same public workflow as `e2b-docs/sdk`.
- Known unsupported area: snapshot APIs are not exposed because the underlying platform does not support them yet.
- Go-specific note: the SDK aims for semantic equivalence rather than identical hand feel. Method names, `context.Context`, and `(..., error)` returns stay Go-native on purpose.
- Runtime compatibility note: the SDK normalizes a few runtime-specific quirks so the high-level behavior stays E2B-like, such as missing-process `Kill()` results and PTY reconnect output framing.

## Environment

Use environment variables for gateway configuration in all examples and quick starts:

- `SEAINFRA_BASE_URL`: SeaInfra gateway entrypoint
- `SEAINFRA_API_KEY`: API key used for gateway routing and authentication
- `SEAINFRA_PROJECT_ID`: optional project routing key for project-scoped gateways
- `SEAINFRA_TEMPLATE_ID`: sandbox template identifier or official template type for your target environment

Set them once in your shell:

```bash
export SEAINFRA_BASE_URL="https://sandbox-gateway.cloud.seaart.ai"
export SEAINFRA_API_KEY="..."
export SEAINFRA_PROJECT_ID="project-..."
export SEAINFRA_TEMPLATE_ID="tpl-..."
```

Default production gateway:

```text
https://sandbox-gateway.cloud.seaart.ai
```

Use `SEAINFRA_TEMPLATE_ID` for production integrations. It can be either a concrete template ID such as `tpl-...` or a stable official template type such as `base`, `claude`, or `codex` when your environment publishes those official templates.

## Production Readiness

- Initialize exactly one root client per process and reuse it.
- Treat every quick start as creating billable or quota-bound resources unless it explicitly cleans them up.
- Prefer explicit template references from configuration over hardcoded example values.
- In SeaInfra environments, prefer official template types such as `base`, `claude`, or `codex` when you want a stable platform-managed entrypoint.
- Use longer client timeouts for `waitReady` flows and image builds.
- Derive runtime access from sandbox responses instead of storing runtime endpoints or tokens in config.

## Compatibility

- Go: see `go.mod` for the supported toolchain version used by this SDK.
- API model: this SDK targets the unified SeaInfra sandbox gateway and keeps public template APIs limited to user-facing fields.
- Stability: operator/admin routes may exist on the gateway, but they are not part of the public SDK workflow described in this README.
- Retry model: treat create/delete/build operations as remote control-plane actions; add idempotency and retry policy in your application layer according to your workload.

## Quick Start

### Control Plane

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/core"
)

func main() {
	client, err := sandbox.NewClient(
		os.Getenv("SEAINFRA_BASE_URL"),
		os.Getenv("SEAINFRA_API_KEY"),
		core.WithProjectID(os.Getenv("SEAINFRA_PROJECT_ID")),
		core.WithTimeout(180*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	timeout := int32(1800)
	createdSandbox, err := client.Create(context.Background(), os.Getenv("SEAINFRA_TEMPLATE_ID"), &sandbox.CreateOptions{
		WaitReady: &ready,
		Timeout:   &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer createdSandbox.Delete(context.Background())

	log.Printf("sandbox=%s envd=%v", createdSandbox.SandboxID, createdSandbox.EnvdURL)
	if createdSandbox.EnvdURL != nil {
		runtime, err := createdSandbox.Runtime()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("runtime=%s", runtime.BaseURL())
	}
}
```

### Bound Sandbox Workflow

```go
client, err := sandbox.NewClient(os.Getenv("SEAINFRA_BASE_URL"), os.Getenv("SEAINFRA_API_KEY"))
if err != nil {
	log.Fatal(err)
}

listed, err := client.List(context.Background(), nil)
if err != nil {
	log.Fatal(err)
}

for _, sandbox := range listed {
	detail, err := sandbox.Reload(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox=%s status=%s", detail.SandboxID, detail.Status)
}
```

### Template Build

```go
template := sandbox.NewTemplate().
	FromImage("docker.io/library/alpine:3.20").
	RunCmd("echo hello-from-go >/tmp/hello.txt", nil).
	SetReadyCmd(sandbox.WaitForFile("/tmp/hello.txt"))

client, err := sandbox.NewClient(os.Getenv("SEAINFRA_BASE_URL"), os.Getenv("SEAINFRA_API_KEY"))
if err != nil {
	log.Fatal(err)
}

built, err := client.BuildTemplate(context.Background(), template, "demo:v1", nil)
if err != nil {
	log.Fatal(err)
}

log.Printf("template=%s build=%s status=%s", built.TemplateID, built.BuildID, built.Status)
```

High-level template helpers currently include:

- lifecycle and status: `client.BuildTemplate`, `client.BuildTemplateInBackground`, `client.TemplateExists`, `client.TemplateAliasExists`, `client.GetTemplateBuildStatus`, `client.ListTemplates`, `client.GetTemplate`, `client.DeleteTemplate`
- serialization: `sandbox.TemplateToJSON`, `sandbox.TemplateToDockerfile`
- base images and registries: `FromDockerfile`, `FromBaseImage`, `FromNodeImage`, `FromPythonImage`, `FromBunImage`, `FromUbuntuImage`, `FromDebianImage`, `FromAWSRegistry`, `FromGCPRegistry`
- build-step helpers: `Copy`, `CopyItems`, `SkipCache`, `AptInstall`, `GitClone`, `MakeDir`, `MakeSymlink`, `NpmInstall`, `PipInstall`, `BunInstall`, `Remove`, `Rename`, `RunCmd`, `RunCmds`
- execution and config helpers: `SetEnvs`, `SetWorkdir`, `SetUser`, `SetStartCmd`, `SetReadyCmd`, `FilesHash`
- supported local copy options: `ForceUpload`, `Mode`, `ResolveSymlinks`
- supported command and path options: `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`, `GitClone(..., &sandbox.TemplateGitCloneOptions{User: ...})`, `MakeDir(..., &sandbox.TemplateMakeDirOptions{User: ...})`, `MakeSymlink(..., &sandbox.TemplateMakeSymlinkOptions{User: ...})`, `Remove(..., &sandbox.TemplateRemoveOptions{...User: ...})`, `Rename(..., &sandbox.TemplateRenameOptions{...User: ...})`
- intentionally not exposed yet: `Copy(..., user=...)`, MCP server helpers, and devcontainer helpers

### Raw Build Plane Through Root Client

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/SeaArt-Infra/sandbox-go"
	"github.com/SeaArt-Infra/sandbox-go/build"
)

func main() {
	client, err := sandbox.NewClient(
		os.Getenv("SEAINFRA_BASE_URL"),
		os.Getenv("SEAINFRA_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Build.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name: "demo",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Build.DeleteTemplate(context.Background(), resp.TemplateID)

	buildID := "build-demo"
	_, err = client.Build.CreateBuild(
		context.Background(),
		resp.TemplateID,
		buildID,
		build.NewTemplateBuildBuilder().
			FromImage("docker.io/library/alpine:3.20").
			Run("echo hello-from-go >/tmp/hello.txt", nil).
			Request(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("template=%s build=%s", resp.TemplateID, buildID)
}
```

### Runtime Modules

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/SeaArt-Infra/sandbox-go"
)

func main() {
	client, err := sandbox.NewClient(os.Getenv("SEAINFRA_BASE_URL"), os.Getenv("SEAINFRA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	createdSandbox, err := client.Create(context.Background(), os.Getenv("SEAINFRA_TEMPLATE_ID"), &sandbox.CreateOptions{
		WaitReady: &ready,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer createdSandbox.Delete(context.Background())

	files, err := createdSandbox.Files()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := files.Write(context.Background(), "/root/workspace/hello.txt", []byte("hello from go")); err != nil {
		log.Fatal(err)
	}

	body, err := files.Read(context.Background(), "/root/workspace/hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s", body)
}
```

Bound sandbox helpers currently include:

- lifecycle: `Reload`, `Connect`, `Resume`, `GetInfo`, `Logs`, `Pause`, `Kill`, `Delete`, `Refresh`, `SetTimeout`, `IsRunning`
- runtime conveniences: `GetMetrics`, `GetHost`, `Proxy`
- commands module: `Run`, `Exec`, `Start`, `Wait`, `List`, `Connect`, `Kill`, `SendStdin`
- filesystem module: `Exists`, `GetInfo`, `List`, `MakeDir`, `Read`, `Write`, `WriteFiles`, `Remove`, `Rename`, `WatchDir`
- git module: `Clone`, `Pull`, `Checkout`, `Status`
- pty module: `Create`, `Connect`, `Kill`, `SendStdin`, `Resize`

## Recommended Usage

For most integrations, prefer one root client per process:

- initialize once with `sandbox.NewClient(baseURL, apiKey, opts...)`
- create sandboxes with `client.Create(...)`
- continue through `Commands()/Files()/Git()/Pty()`
- build templates with `sandbox.NewTemplate()` plus `client.BuildTemplate(...)`

Low-level methods remain available when you need tighter request control:

- use `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- continue from the returned sandbox object with `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, and `IsRunning()`
- only switch to runtime with `Runtime()` when you need file/process/stream operations
- use `BuildTemplate`, `BuildTemplateInBackground`, `TemplateExists`, `GetTemplateBuildStatus`, `ListTemplates`, `GetTemplate`, and `DeleteTemplate` on the root client for bound template workflows
- use `client.Build` only for raw template/build workflows
- use `build.NewTemplateBuildBuilder()` when you want a small fluent helper that expands into `BuildRequest`

Low-level domain packages remain available when you need direct request/response models or tighter transport control.

## API Surface

### Control Plane APIs

`sandbox.Client` exposes control-plane methods directly:

- system: `Metrics`, `Shutdown`
- sandboxes: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `DeleteSandbox`
- sandbox operations: `GetSandboxLogs`, `PauseSandbox`, `ConnectSandbox`, `SetSandboxTimeout`, `RefreshSandbox`, `SendHeartbeat`

Recommended root-client path:

- high-level lifecycle: `Create`, `Connect`, `List`, `Get`
- template helpers: `BuildTemplate`, `BuildTemplateInBackground`, `TemplateExists`, `TemplateAliasExists`, `GetTemplateBuildStatus`, `ListTemplates`, `GetTemplate`, `DeleteTemplate`
- low-level lifecycle: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- follow-up control actions from the returned object: `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, `IsRunning()`
- runtime actions from objects that include `EnvdURL`: `Runtime()`

Low-level direct methods like `DeleteSandbox` and `GetSandboxLogs` remain available on the root client when you want explicit control-plane requests.

### Operator APIs

The root client also includes operator-oriented methods such as `GetPoolStatus`, `StartRollingUpdate`, `GetRollingUpdateStatus`, and `CancelRollingUpdate`.

These routes are intended for platform operators, not normal application workloads. Keep them out of business-facing integrations unless you are explicitly building operational tooling.

### Template Facade

Preferred template path:

- `sandbox.NewTemplate()` for build DSL
- `client.BuildTemplate(...)` for create + build + optional polling
- `client.BuildTemplateInBackground(...)` for fire-and-poll-later workflows
- `client.ListTemplates(...)`, `client.GetTemplate(...)`, `client.DeleteTemplate(...)`, `client.TemplateExists(...)`, `client.TemplateAliasExists(...)`, `client.GetTemplateBuildStatus(...)` for bound lifecycle and status
- `sandbox.TemplateToJSON(...)`, `sandbox.TemplateToDockerfile(...)` for export helpers

Template builder conveniences include:

- base images and registries: `FromDockerfile` (returns `(*Template, error)`), `FromBaseImage`, `FromNodeImage`, `FromPythonImage`, `FromBunImage`, `FromUbuntuImage`, `FromDebianImage`, `FromAWSRegistry`, `FromGCPRegistry`
- file and command helpers: `Copy`, `CopyItems`, `SkipCache`, `AptInstall`, `GitClone`, `MakeDir`, `MakeSymlink`, `NpmInstall`, `PipInstall`, `BunInstall`, `Remove`, `Rename`, `RunCmd`, `RunCmds`
- execution and config helpers: `SetEnvs`, `SetWorkdir`, `SetUser`, `SetStartCmd`, `SetReadyCmd`, `FilesHash`
- supported local copy options: `ForceUpload`, `Mode`, `ResolveSymlinks`
- supported command and path options: `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`, `GitClone(..., &sandbox.TemplateGitCloneOptions{User: ...})`, `MakeDir(..., &sandbox.TemplateMakeDirOptions{User: ...})`, `MakeSymlink(..., &sandbox.TemplateMakeSymlinkOptions{User: ...})`, `Remove(..., &sandbox.TemplateRemoveOptions{...User: ...})`, `Rename(..., &sandbox.TemplateRenameOptions{...User: ...})`
- intentionally not exposed yet: `Copy(..., user=...)`, MCP server helpers, and devcontainer helpers

### Build Plane Through `client.Build`

Low-level `client.Build` exposes:

- system: `Metrics`
- direct build: `DirectBuild`
- templates: `CreateTemplate`, `ListTemplates`, `GetTemplateByAlias`, `ResolveTemplateRef`, `GetTemplate`, `UpdateTemplate`, `DeleteTemplate`
- builds: `CreateBuild`, `GetBuildFile`, `RollbackTemplate`, `ListBuilds`, `GetBuild`, `GetBuildStatus`, `GetBuildLogs`

The public template contract is split into three layers: top-level create fields (`Name`, `Tags`, `CPUCount`, `MemoryMB`), SeaInfra template extensions under the protocol-compatible `Extensions.Seacloud` field (`BaseTemplateID`, `Visibility`, `Envs`, `StorageType`, `StorageSizeGB`), and build-only fields on `CreateBuild` (`FromImage`, `FromTemplate`, `Steps`, `StartCmd`, `ReadyCmd`, registry credentials, `FilesHash`).
Public create/update calls reject legacy top-level write fields such as `Alias`, `TeamID`, and `Public`.

For Go callers, the public write path and template read path now use different extension models on purpose:

- `TemplateCreateRequest` / `TemplateUpdateRequest` use `PublicTemplateExtensions`
- `ListedTemplate` / `TemplateResponse` keep the fuller `TemplateExtensions` shape returned by the service

This matches the current public builder API contract: request fields are intentionally narrower than response fields.

`CreateTemplate` and `UpdateTemplate` reject `visibility=official` on public routes, including `Extensions.Seacloud.Visibility == "official"`.

`CreateBuild` now follows the wire contract directly: callers pass top-level `FilesHash` when needed, and the SDK returns the raw `202 {}` trigger response without adding helper fields.

`GetTemplateByAlias` is a pure alias lookup endpoint. It should only be used with an actual published alias value.

`ResolveTemplateRef` is the SeaInfra stable-ref lookup endpoint. It resolves a template by `templateID`, official template `type`, or visible alias.

### Runtime Helper

Low-level runtime objects returned by `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` expose:

- system: `Metrics`, `Envs`, `Configure`, `Ports`
- proxy and file transfer: `Proxy`, `Download`, `FilesContent`, `UploadBytes`, `UploadJSON`, `UploadMultipart`, `WriteBatch`, `ComposeFiles`, `ReadFile`, `WriteFile`
- filesystem RPC: `ListDir`, `Stat`, `MakeDir`, `Remove`, `Move`, `Edit`
- watchers: `WatchDir`, `CreateWatcher`, `GetWatcherEvents`, `RemoveWatcher`
- process RPC: `Start`, `Connect`, `ListProcesses`, `SendInput`, `SendSignal`, `CloseStdin`, `Update`, `StreamInput`, `GetResult`, `Run`

`cmd.RequestOptions` supports:

- `Username`: basic-auth username for envd operations that need it
- `Signature` and `SignatureExpiration`: signed file access parameters
- `Range`: partial download support
- `Headers`: custom header injection

Streaming APIs return `ProcessStream`, `FilesystemWatchStream`, and `ConnectFrame`.

## Resource Safety

- The quick starts are written for disposable resources and should be adapted before copy-pasting into production jobs.
- Prefer explicit cleanup with `defer createdSandbox.Delete(...)` and `defer client.Build.DeleteTemplate(...)` when running probes, smoke tests, or CI.
- For long-lived workloads, move cleanup and timeout policy into your own lifecycle manager instead of relying on sample code defaults.

## Package Layout

- `github.com/SeaArt-Infra/sandbox-go`: root client and recommended entrypoint
- `github.com/SeaArt-Infra/sandbox-go/control`: control-plane models and low-level APIs
- `github.com/SeaArt-Infra/sandbox-go/build`: build-plane models and low-level APIs
- `github.com/SeaArt-Infra/sandbox-go/cmd`: runtime models and low-level APIs
- `github.com/SeaArt-Infra/sandbox-go/core`: shared transport and error primitives

## Notes

- The gateway entrypoint always needs `baseURL + apiKey` to initialize.
- Project-scoped deployments can set `core.WithProjectID(...)`; the SDK sends `X-Project-ID` on control/build requests.
- Runtime access should be derived from sandbox response objects with `Runtime()` or `RuntimeFromSandbox(...)`.
- `CreateSandbox` and `GetSandbox` responses include `EnvdURL` and `EnvdAccessToken` when the target sandbox supports CMD access.
- Runtime file/process APIs require a template image that starts nano-executor and returns runtime access fields; if runtime APIs return `404`, verify the selected template supports CMD runtime routes.
- `waitReady=true` can take longer than the default HTTP timeout in production; pass `core.WithTimeout(...)` when creating long-wait clients.
- API errors expose `Kind` and `Retryable()` for retry logic and alert routing.
- High-level `Kill()` helpers send `SIGNAL_SIGKILL` and return `false` when the runtime reports a missing process through either `404` or `ESRCH`.
- PTY handles normalize reconnect output into `PTY` even when the runtime emits the bytes through `Stdout` / `Stderr`.
- Sandbox timeout values are validated to `0..86400`; refresh duration to `0..3600`.
- Build request validation accepts E2B-style `COPY` / `ENV` / `RUN` / `WORKDIR` / `USER` steps, `force`, and structured `fromImageRegistry` credentials (`registry` / `aws` / `gcp`).
- Some gateways do not expose `/admin/*` or `/build`; integration tests skip those cases on `404`.
- Some filesystem layouts reject watcher APIs entirely; the integration suite skips watcher coverage when the runtime reports that limitation.

## Security

- Do not commit `SEAINFRA_API_KEY`, `envdAccessToken`, or sandbox access tokens.
- Treat runtime tokens as sandbox-scoped secrets. Prefer `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` so response-scoped runtime access is not copied into configuration.
- Do not log raw API keys or runtime tokens. SDK errors may include response bodies, so avoid logging full error payloads in shared systems.
- Set `core.WithProjectID(...)` when your gateway requires explicit project routing. The SDK sends it as `X-Project-ID`.

## Production Smoke

Use production smoke tests only with explicitly provided credentials and disposable sandboxes:

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEAINFRA_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEAINFRA_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests/... -run Integration -count=1 -v
```

`tpl-base-dc11799b9f9f4f9e` is a known-good SeaInfra runtime template for validating CMD routes such as `ListDir`, `ReadFile`, `WriteFile`, and `Run`.
You can also run the same disposable smoke flow from GitHub Actions with `.github/workflows/integration-smoke.yml` after setting the `SANDBOX_TEST_API_KEY` repository secret.

## Integration Tests

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEAINFRA_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEAINFRA_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=... \
go test ./tests/... -count=1 -v
```

Use a runtime-enabled template for CMD integration coverage. For SeaInfra production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good runtime template.
The same smoke suite is available as a manual GitHub Actions dispatch in `.github/workflows/integration-smoke.yml`.

## Release

- See `CHANGELOG.md` for release notes.
- See `RELEASE_CHECKLIST.md` before tagging or publishing a new version.
