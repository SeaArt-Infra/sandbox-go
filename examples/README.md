# Go Examples

Run examples from the package root.

Shared env:

- `SEAINFRA_BASE_URL`
- `SEAINFRA_API_KEY`

Before running any example, export these variables once in your shell. Use the gateway entrypoint documented in the root `README.md`.

Example-specific inputs intentionally use the `SANDBOX_EXAMPLE_*` prefix so they do not collide with the production-oriented variables shown in the package root `README.md`.
Examples focus on the stable lifecycle, template, command, and PTY flows. Watcher APIs are covered in tests instead, because some sandbox filesystem layouts reject them entirely.

Recommended reading order:

1. `full_workflow`: create a template -> trigger a build -> wait for build -> start sandbox -> connect runtime -> run -> logs/metrics -> cleanup
2. `template_features`: `FromDockerfile` -> local `Copy(..., Mode/ResolveSymlinks)` -> `client.BuildTemplateInBackground(...)` -> `client.GetTemplateBuildStatus(...)` -> existence/detail
3. `control_sandbox`: `sandbox.NewClient(...)` -> `client.Create(...)` -> reload -> cleanup
4. `cmd_smoke`: `sandbox.NewClient(...)` -> `client.Create(...)` -> `Files()` / `Commands()`
5. `build_template`: minimal `sandbox.NewTemplate()` plus `client.BuildTemplate(...)`

## Full Workflow

This is the primary example when evaluating the SDK end to end:

- create a template
- trigger a build from a runtime-enabled base image
- wait for the build to finish
- inspect build status, build logs, and template detail
- start a sandbox from that template
- reload, fetch sandbox logs, connect, inspect runtime metrics, and run a command
- delete the sandbox and template unless `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

Required env:

- `SANDBOX_EXAMPLE_RUNTIME_BASE_IMAGE`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

The base image must already be runtime-enabled for CMD APIs.

```bash
go run ./examples/full_workflow
```

## Control Plane

This example shows the preferred workflow:

- initialize one root client
- create a sandbox through `client.Create(...)`
- keep operating through the returned bound sandbox object
- reload once to show the bound-object workflow
- cleanup through the same object

Required env:

- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/control_sandbox
```

## Build Plane

Recommended path: the example uses `sandbox.NewTemplate()` plus `client.BuildTemplate(...)`.
The flow shows the current client-first template workflow directly: template DSL -> build polling -> template detail -> cleanup.

Required env: none

Optional env:

- `SANDBOX_EXAMPLE_BUILD_IMAGE`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/build_template
```

## Template Features

This example covers the supported template helpers that are not obvious from the minimal build flow:

- parse a Dockerfile from disk with `FromDockerfile`
- inspect the generated request with `sandbox.TemplateToJSON(...)` and `sandbox.TemplateToDockerfile(...)`
- add extra steps with `SkipCache()` and `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`
- upload a local symlink target with `Copy(..., &sandbox.TemplateCopyOptions{Mode, ResolveSymlinks})`
- initialize one root client
- trigger `client.BuildTemplateInBackground(...)` and poll with `client.GetTemplateBuildStatus(...)`
- verify template existence and inspect template detail

Required env: none

Optional env:

- `SANDBOX_EXAMPLE_BUILD_IMAGE`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/template_features
```

## CMD Plane

Recommended path: the example uses `client.Create(...)` and then stays on `Files()` / `Commands()`.
The selected template must include nano-executor runtime support; otherwise file/process/RPC calls can return `404`.
The flow stays minimal: write file -> read file -> list directory -> run command.
The example writes under `/root/workspace`, which is the writable sandbox workspace in the current SeaInfra runtime.

Required env:

- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/cmd_smoke
```

For SeaInfra production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good template to use when creating the runtime-enabled sandbox.
