# Cross-Platform Release Design

Date: 2026-07-17
Status: Approved
Scope: `cmd/kubectl-cce` cross-platform support + GitHub Releases distribution

## Goals

1. Make the plugin run on **Windows and Linux**, on **ARM64 and AMD64**.
2. Publish versioned releases via **GoReleaser**, producing downloadable
   binaries attached to GitHub Releases.
3. Keep the existing core (local reverse proxy + AK/SK signing) unchanged.

## Non-goals

- macOS support (not requested).
- krew plugin manifest (separate review process, not requested).
- Changes to the signing / proxy logic.
- Handling the user's `kubectl` dependency (users must install `kubectl`
  themselves; this is the standard kubectl plugin convention).

## Current state and gap

- Core proxy + signing works and is tested (`cmd/kubectl-cce/main.go`).
- `main.go:426` hard-codes `--kubeconfig=/dev/null`, a Unix path that breaks
  the plugin on Windows. This is the only Windows blocker.
- No `version` variable and no `--version` flag.
- No `.github/workflows/`, no `.goreleaser.yml`, no `.gitattributes`.
- CRLF checkouts on Windows cause `gofmt -l .` to flag every file.

## Architecture

The runtime flow is unchanged. Three independent, separately-testable
capabilities are added around the existing core:

```
parse flags
  └─ (new) if --version -> print version, exit 0
loadConfig + validate
newProxy (127.0.0.1:0, signs upstream requests)
  └─ (new) tempKubeconfigPath() -> temp empty kubeconfig file
runKubectlThroughProxy(--kubeconfig=<tempfile>)
  └─ (changed) was --kubeconfig=/dev/null
cleanup temp file
shut proxy down
```

## Components

### A. Version injection — `cmd/kubectl-cce/main.go`

- Add package-level `var version = "dev"`.
- Add a `--version` bool flag to the `run()` FlagSet. When set, print
  `version` to stdout and `return nil` (exit 0) before starting the proxy.
- GoReleaser injects the release version via ldflags:
  `-ldflags "-X main.version={{ .Version }}"`.

### B. Cross-platform kubeconfig — `cmd/kubectl-cce/main.go`

- Add `func tempKubeconfigPath() (path string, cleanup func(), err error)`.
  - Uses `os.CreateTemp` to write a minimal valid empty kubeconfig:
    `apiVersion: v1\nkind: Config\n`.
  - Returns the path and a `cleanup` closure that removes the file
    (best-effort; failures only logged when `CCE_PROXY_DEBUG=1`).
- In `runKubectlThroughProxy` (`main.go:421`), replace the
  `--kubeconfig=/dev/null` arg with `--kubeconfig=<temp path>` and `defer`
  the cleanup.
- Rationale: `os.DevNull` (`NUL` on Windows) makes kubectl read empty
  bytes that may be rejected as invalid YAML; a real temp file with a valid
  empty `Config` is robust and uniform across platforms.

### C. GoReleaser config — `.goreleaser.yml`

- `version: 2`.
- `builds`: one build, binary name `kubectl-cce`, ldflags inject
  `main.version`, explicit `targets`:
  `linux_amd64`, `linux_arm64`, `windows_amd64`, `windows_arm64`
  (no cgo, pure stdlib cross-compile).
- `archives`: `name_template`
  `kubectl-cce_{{ .Version }}_{{ .Os }}_{{ .Arch }}`; Linux = `tar.gz`,
  Windows = `zip`.
- `checksum`: file `checksums.txt` (SHA256).
- `release`: repo `pancake0001/kubectl-cce-plugin`.
- `changelog`: auto-generated.

### D. GitHub Actions — `.github/workflows/`

- `release.yml`: trigger on `push tags v*`; steps: checkout, setup-go,
  GoReleaser release using the auto-provided `GITHUB_TOKEN`.
- `ci.yml`: trigger on push/PR; runs `gofmt -l .` (clean because of LF
  checkout), `go vet ./...`, `go test ./...`.

### E. `.gitattributes`

```
* text=auto eol=lf
```

Forces LF checkout on all platforms, eliminating the Windows `gofmt -l`
CRLF false positives and making the CI format check trustworthy.

### F. README — Install section

Rewrite the Install section so the primary path is downloading a binary
from GitHub Releases:

1. Download `kubectl-cce_<version>_<os>_<arch>.tar.gz` / `.zip`.
2. Extract and rename to `kubectl-cce` (Linux) or `kubectl-cce.exe`
   (Windows).
3. Put it on `PATH`.
4. Verify with `kubectl plugin list`.

Keep source `go build` as a secondary path. Document `kubectl cce
--version` for version checking.

## Data flow

```
run(args)
  -> fs.Parse
  -> if --version: fmt.Println(version); return nil
  -> loadConfig(); cfg.validate()
  -> newProxy(cfg)
  -> runKubectlThroughProxy(proxy, cfg, kubectlArgs)
       -> path, cleanup, err := tempKubeconfigPath()
       -> args = [..., "--kubeconfig=" + path]
       -> defer cleanup()   // runs when runKubectlThroughProxy returns
       -> cmd.Run()
  -> defer proxy.close()
```

## Error handling

- `tempKubeconfigPath` creation failure -> returned as error; proxy (if up)
  is shut down via `defer proxy.close()`.
- Temp file removal failure -> best-effort; logged to stderr only when
  `CCE_PROXY_DEBUG=1`; never affects the exit code.
- `--version` -> exit 0.

## Testing

- `TestTempKubeconfigValidAndCrossPlatform`: path is non-empty, content
  parses as a valid empty `Config`, file is gone after `cleanup()`.
- `TestVersionFlagPrintsAndExits`: `run(["--version"])` prints the version
  and returns `nil`.
- All existing tests (`main_test.go`) remain green.
- CI: `go vet ./...` + `go test ./...` + `gofmt -l .` (clean via
  `.gitattributes`).

## Release process

1. Bump version, commit.
2. `git tag v<version> && git push origin v<version>`.
3. `release.yml` triggers GoReleaser: builds 4 targets, archives, checksums,
   creates the GitHub Release, uploads assets.
4. Users download from the Releases page.
