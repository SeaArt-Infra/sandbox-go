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
5. `build_template`: resolve Node base -> upload local directory -> build -> create sandbox -> verify COPY -> cleanup
6. `build_nfs_web_template`: derive from the managed NFS base -> upload a local Web app -> initialize the NFS workspace -> run Web and executor services together -> verify pause/resume persistence

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

Recommended path: the example resolves the managed Node template, adds the local
`examples/build_template/context` directory with `Template.Copy`, builds a personal
template, starts a sandbox from it, and reads the copied marker through the runtime
command API. The sandbox and template are deleted in that order after verification.

Required env: none

Optional env:

- `SANDBOX_EXAMPLE_BASE_TEMPLATE` (default: `node`)
- `SANDBOX_EXAMPLE_BUILD_CONTEXT` (default: `examples/build_template/context`;
  custom contexts must contain `sandbox-go-build-context.txt` with
  `sandbox-go-copy-ok`)
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/build_template
```

## NFS Web Template

This example builds the default-start Web template used with the managed NFS
runtime. It resolves the official `nfs` template, uploads a local npm/Vite app,
installs Node.js in the derived image, and builds the app. At Sandbox startup it:

- overrides the inherited workspace mount with an operator-provided NFS host path
- copies `/app` into `/agent-workspace` only when the NFS workspace is empty
- sets the runtime workdir to the NFS mount so executor file APIs use the persistent root
- starts the Web app on port `3000`
- leaves the managed nano-executor running on port `9000`
- verifies the Web proxy and executor APIs
- writes an NFS marker, pauses and resumes the same Sandbox, and verifies that the marker remains
- deletes the Sandbox and derived template after verification

The local source must contain `package.json`, `package-lock.json`, `index.html`,
and `src`, with `build` and `dev` npm scripts. The example explicitly uploads
the standard source/config files instead of copying the entire local directory,
so local `.env`, `.git`, `node_modules`, and generated output are not included.

Required env:

- `SANDBOX_EXAMPLE_WEB_SOURCE_DIR`
- `SANDBOX_EXAMPLE_NFS_HOST_PATH` (the NFS root assigned by the platform operator)

Optional env:

- `SANDBOX_EXAMPLE_NFS_BASE_TEMPLATE` (default: `nfs`)
- `SANDBOX_EXAMPLE_NFS_WORKSPACE_DIR` (default: `/agent-workspace`)
- `SANDBOX_EXAMPLE_TEMPLATE_NAME` (default: generated unique name)
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/build_nfs_web_template
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
