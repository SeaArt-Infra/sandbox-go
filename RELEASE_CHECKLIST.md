# Release Checklist

Use this checklist before tagging and publishing a release.

## Preflight

- Confirm `README.md`, `CHANGELOG.md`, and examples match the released API.
- Confirm no API keys, access tokens, or production URLs are committed.
- Run `go test ./...`.
- Run production smoke only with an explicitly provided API key and a runtime-enabled template.

## Production Smoke

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEAINFRA_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEAINFRA_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests -run Integration -v
```

## Tag

- Update `core.SDKVersion` if the release should embed a fixed SDK version.
- Create a signed tag, for example `git tag -s v0.1.0 -m "v0.1.0"`.
- Push `main` and the tag.
