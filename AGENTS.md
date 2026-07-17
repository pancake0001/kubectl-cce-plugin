# AGENTS.md

## Project

`kubectl-cce` is a Go kubectl plugin (stdlib only, no external deps) that
starts a short-lived local reverse proxy, signs each upstream request with
Huawei Cloud AK/SK (`SDK-HMAC-SHA256`), and shells out to the real `kubectl`
pointed at the proxy. Single package, single entrypoint:
`cmd/kubectl-cce/main.go`.

Runtime flow: parse flags -> load config from env -> start HTTP proxy on
`127.0.0.1:0` -> run
`kubectl --server=<proxy> --insecure-skip-tls-verify --kubeconfig=<tempfile> <args>`
-> shut proxy down.

## Commands

- Build: `go build -o kubectl-cce ./cmd/kubectl-cce`
- Test: `go test ./...` (one package, no services or fixtures required)
- Vet: `go vet ./...`
- Format check: `gofmt -l .`
- Version: `kubectl cce --version` (prints `dev` for local builds; the tag
  version for release builds via ldflags)

## Toolchain

- Go 1.25.x (matches the `go 1.25.0` directive in `go.mod`).
- CI: GitHub Actions — `.github/workflows/ci.yml` runs gofmt/vet/test +
  cross-compile on push/PR; `release.yml` runs GoReleaser on `v*` tags.
- Releases: GoReleaser (`.goreleaser.yml`) builds linux/windows ×
  amd64/arm64, produces archives + `checksums.txt`, and publishes to GitHub
  Releases. Publish a new version with
  `git tag v<ver> && git push origin v<ver>`.
- No `golangci-lint`; `go vet` + `gofmt` are the local checks.
- No `go.sum`; the module has zero external runtime dependencies.

## Gotchas

- **Binary name matters**: the built executable must be named `kubectl-cce`
  (`.exe` on Windows) for kubectl plugin discovery (`kubectl plugin list`).
- **Blocked streaming commands**: `exec`, `attach`, `port-forward` are
  rejected by `unsupportedStreamingCommands` in `main.go` because the CCE
  API Gateway does not pass websocket/SPDY upgrades.
- **Auth precedence**: AK/SK signing is used when both
  `HUAWEICLOUD_SDK_AK` and `HUAWEICLOUD_SDK_SK` are set; otherwise the
  plugin falls back to `HUAWEI_IAM_TOKEN` (`X-Auth-Token`). Set
  `CCE_PROXY_DEBUG=1` to dump the canonical request + string-to-sign to
  stderr.
