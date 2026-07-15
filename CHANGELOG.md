# Changelog

All notable changes to this project will be documented in this file.

This project follows Semantic Versioning for public SDK APIs.

## [0.1.3] - 2026-05-08

### Fixed

- Aligned high-level command and PTY kill helpers with the runtime signal enum by sending `SIGNAL_SIGKILL`.
- Normalized high-level `Kill()` helpers to return `false` for both `404` and runtime `ESRCH` missing-process responses.
- Added PTY wait fallback so reconnect output still lands in `PTY` when the runtime emits it through `Stdout` / `Stderr`.

### Changed

- Added a manual GitHub Actions integration-smoke workflow for disposable real-environment validation.
- Documented watcher filesystem limitations and high-level runtime normalization behavior.

## [0.1.2] - 2026-04-24

### Changed

- Refined README and examples around the unified gateway flow and environment-based configuration.
- Added a full end-to-end workflow example covering template creation, sandbox startup, runtime execution, and cleanup.
- Reduced build request surface to the user-facing fields needed for production SDK usage.
- Expanded the high-level template builder toward the E2B design for supported features: build/status helpers, tags, Dockerfile import/export, image helpers, `CopyItems`, `SkipCache`, `RunCmd` user options, and local COPY tar options (`Mode`, `ResolveSymlinks`).

## [0.1.0] - 2026-04-23

### Added

- Initial Go SDK for SeaInfra sandbox control-plane, build-plane, and runtime CMD APIs.
- Unified root client initialization with `sandbox.NewClient(baseURL, apiKey)`.
- Build namespace through `client.Build`.
- Runtime helpers through `client.Runtime(...)`, `client.RuntimeFromSandbox(...)`, and bound sandbox objects.
- Typed API errors with retry classification.
- Configurable HTTP timeout through `core.WithTimeout(...)`.
- Examples, unit tests, and integration-test scaffolding.
