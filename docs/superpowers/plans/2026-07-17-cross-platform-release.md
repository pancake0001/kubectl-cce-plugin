# Cross-Platform Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `kubectl-cce` run on Windows+Linux / ARM64+AMD64 and publish downloadable versioned binaries via GoReleaser on GitHub Releases.

**Architecture:** The core proxy+signing flow is untouched. Three independent capabilities are layered around it: (1) a `--version` flag backed by an ldflags-injected `version` var, (2) a cross-platform temp kubeconfig replacing the Unix-only `/dev/null`, (3) GoReleaser config + GitHub Actions + `.gitattributes` for release distribution and clean formatting.

**Tech Stack:** Go (stdlib only), GoReleaser v2, GitHub Actions. No new runtime dependencies.

**Spec:** `docs/superpowers/specs/2026-07-17-cross-platform-release-design.md`

---

## File Structure

| File | Action | Responsibility |
| --- | --- | --- |
| `.gitattributes` | Create | Enforce LF line endings everywhere (kills Windows gofmt noise) |
| `cmd/kubectl-cce/main.go` | Modify | Add `version` var, `--version` flag, `tempKubeconfigPath()`, swap `/dev/null` |
| `cmd/kubectl-cce/main_test.go` | Modify | Tests for `--version` and `tempKubeconfigPath` |
| `.gitignore` | Modify | Ignore GoReleaser `dist/` output |
| `.goreleaser.yml` | Create | 4-target build, archives, checksum, release |
| `.github/workflows/ci.yml` | Create | gofmt + vet + test + cross-compile on push/PR |
| `.github/workflows/release.yml` | Create | GoReleaser release on `v*` tags |
| `README.md` | Modify | Rewrite Install section for binary releases |

---

## Task 1: Enforce LF line endings with `.gitattributes`

**Files:**
- Create: `.gitattributes`

**Why first:** makes `gofmt -l .` trustworthy on Windows so every later task can rely on a clean format check.

- [ ] **Step 1: Create `.gitattributes`**

```
* text=auto eol=lf
```

- [ ] **Step 2: Normalize the working tree to LF**

The current tracked Go files are checked out with CRLF; `gofmt -w` rewrites them to canonical LF (logic is unchanged — earlier `gofmt -d` showed only line-ending diffs). Run:

```powershell
gofmt -w cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go
```

- [ ] **Step 3: Verify gofmt is now clean**

Run: `gofmt -l .`
Expected: no output (empty).

- [ ] **Step 4: Confirm no logic changed**

Run: `git diff --stat cmd/`
Expected: no diff shown (git already stores LF, so after normalization the working tree matches the index). If a diff appears, inspect with `git diff cmd/` — it must be line endings only.

- [ ] **Step 5: Commit**

```powershell
git add .gitattributes
git commit -m "Add .gitattributes to enforce LF line endings"
```

---

## Task 2: Add `--version` flag with ldflags injection (TDD)

**Files:**
- Modify: `cmd/kubectl-cce/main.go` (add `version` var, change `run` signature, add `--version` flag)
- Test: `cmd/kubectl-cce/main_test.go`

- [ ] **Step 1: Write the failing test**

Add imports and this test to `cmd/kubectl-cce/main_test.go`. Replace the existing import block (add `bytes` and `strings`; `os` is added in Task 3):

```go
import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)
```

Append the test:

```go
func TestVersionFlagPrintsAndExits(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--version"}, &stdout); err != nil {
		t.Fatalf("run(--version) err = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/kubectl-cce/ -run TestVersionFlag -v`
Expected: COMPILE ERROR — `run` takes 1 arg (signature mismatch) and `version` is undefined.

- [ ] **Step 3: Implement — add the `version` variable**

In `cmd/kubectl-cce/main.go`, immediately after the `unsupportedStreamingCommands` var block (after `}` on line 38), add:

```go
var version = "dev"
```

- [ ] **Step 4: Implement — change `run` signature and wire the flag**

In `cmd/kubectl-cce/main.go`, replace the `main` function:

```go
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		var exitErr kubectlExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.code)
		}
		fmt.Fprintf(os.Stderr, "kubectl-cce: %v\n", err)
		os.Exit(2)
	}
}
```

Replace the `run` function header and the parse+version block. Find:

```go
func run(args []string) error {
	fs := flag.NewFlagSet("kubectl cce", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	printProxyURL := fs.Bool("print-proxy-url", false, "print a temporary local proxy URL and exit")
	insecureTLS := fs.Bool("cce-insecure-upstream-tls", false, "skip TLS verification for the upstream CCE endpoint")
	clusterID := fs.String("cluster-id", "", "CCE cluster ID; overrides CCE_CLUSTER_ID")
	clusterAlias := fs.String("cluster", "", "alias of --cluster-id")
	region := fs.String("region", "", "Huawei Cloud region; overrides CCE_REGION")
	endpoint := fs.String("endpoint", "", "CCE API Gateway endpoint host; overrides CCE_ENDPOINT")
	projectID := fs.String("project-id", "", "Huawei Cloud project ID; overrides CCE_PROJECT_ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
```

Replace with:

```go
func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("kubectl cce", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	printProxyURL := fs.Bool("print-proxy-url", false, "print a temporary local proxy URL and exit")
	insecureTLS := fs.Bool("cce-insecure-upstream-tls", false, "skip TLS verification for the upstream CCE endpoint")
	clusterID := fs.String("cluster-id", "", "CCE cluster ID; overrides CCE_CLUSTER_ID")
	clusterAlias := fs.String("cluster", "", "alias of --cluster-id")
	region := fs.String("region", "", "Huawei Cloud region; overrides CCE_REGION")
	endpoint := fs.String("endpoint", "", "CCE API Gateway endpoint host; overrides CCE_ENDPOINT")
	projectID := fs.String("project-id", "", "Huawei Cloud project ID; overrides CCE_PROJECT_ID")
	showVersion := fs.Bool("version", false, "print the kubectl-cce version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return nil
	}
```

`io` and `os` are already imported — no new imports needed.

- [ ] **Step 5: Normalize and run the full test suite**

```powershell
gofmt -w cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go
go vet ./...
go test ./... -v
```
Expected: all tests PASS (including `TestVersionFlagPrintsAndExits`).

- [ ] **Step 6: Commit**

```powershell
git add cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go
git commit -m "Add --version flag with ldflags injection"
```

---

## Task 3: Replace `/dev/null` with cross-platform temp kubeconfig (TDD)

**Files:**
- Modify: `cmd/kubectl-cce/main.go` (add `tempKubeconfigPath`, update `runKubectlThroughProxy`)
- Test: `cmd/kubectl-cce/main_test.go`

- [ ] **Step 1: Write the failing test**

First add `os` to the test file's import block (it now reads):

```go
import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
)
```

Append to `cmd/kubectl-cce/main_test.go`:

```go
func TestTempKubeconfigValidAndCrossPlatform(t *testing.T) {
	path, cleanup, err := tempKubeconfigPath()
	if err != nil {
		t.Fatalf("tempKubeconfigPath() err = %v", err)
	}
	if path == "" {
		t.Fatal("tempKubeconfigPath() returned empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp kubeconfig: %v", err)
	}
	want := "apiVersion: v1\nkind: Config\n"
	if string(data) != want {
		t.Fatalf("kubeconfig content = %q, want %q", string(data), want)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove %s: %v", path, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/kubectl-cce/ -run TestTempKubeconfig -v`
Expected: COMPILE ERROR — `tempKubeconfigPath` undefined.

- [ ] **Step 3: Implement — add `tempKubeconfigPath`**

In `cmd/kubectl-cce/main.go`, add this function immediately before `runKubectlThroughProxy` (before line 421):

```go
func tempKubeconfigPath() (string, func(), error) {
	f, err := os.CreateTemp("", "kubectl-cce-kubeconfig-*")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString("apiVersion: v1\nkind: Config\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	path := f.Name()
	cleanup := func() {
		if err := os.Remove(path); err != nil && os.Getenv("CCE_PROXY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "kubectl-cce: failed to remove temp kubeconfig %s: %v\n", path, err)
		}
	}
	return path, cleanup, nil
}
```

- [ ] **Step 4: Implement — use it in `runKubectlThroughProxy`**

In `cmd/kubectl-cce/main.go`, replace the body of `runKubectlThroughProxy`. Find:

```go
func runKubectlThroughProxy(proxy *localProxy, cfg config, kubectlArgs []string) error {
	args := append([]string{
		"--server=" + proxy.url(),
		"--insecure-skip-tls-verify=true",
		"--kubeconfig=/dev/null",
	}, kubectlArgs...)
```

Replace with:

```go
func runKubectlThroughProxy(proxy *localProxy, cfg config, kubectlArgs []string) error {
	kubeconfigPath, cleanup, err := tempKubeconfigPath()
	if err != nil {
		return err
	}
	defer cleanup()

	args := append([]string{
		"--server=" + proxy.url(),
		"--insecure-skip-tls-verify=true",
		"--kubeconfig=" + kubeconfigPath,
	}, kubectlArgs...)
```

Leave the rest of the function (ctx, cmd, Run, error handling) unchanged.

- [ ] **Step 5: Normalize and run tests**

```powershell
gofmt -w cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go
go vet ./...
go test ./... -v
```
Expected: all tests PASS (including `TestTempKubeconfigValidAndCrossPlatform`).

- [ ] **Step 6: Commit**

```powershell
git add cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go
git commit -m "Replace /dev/null with cross-platform temp kubeconfig"
```

---

## Task 4: GoReleaser config

**Files:**
- Create: `.goreleaser.yml`
- Modify: `.gitignore` (add `dist/`)

- [ ] **Step 1: Create `.goreleaser.yml`**

```yaml
version: 2

project_name: kubectl-cce

builds:
  - id: kubectl-cce
    main: ./cmd/kubectl-cce
    binary: kubectl-cce
    ldflags:
      - -s -w -X main.version={{.Version}}
    targets:
      - linux_amd64
      - linux_arm64
      - windows_amd64
      - windows_arm64

archives:
  - id: default
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: pancake0001
    name: kubectl-cce-plugin

changelog:
  use: git
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
```

- [ ] **Step 2: Ignore GoReleaser build output**

Add a `dist/` entry to `.gitignore`. Append (the file already exists; add near the other ignore rules):

```
# GoReleaser build output
dist/
```

- [ ] **Step 3: Smoke-test that the code cross-compiles for all 4 targets**

This needs no goreleaser — plain `go build` verifies the code compiles for every platform (no cgo, pure stdlib):

```powershell
foreach ($t in @(@("linux","amd64"),@("linux","arm64"),@("windows","amd64"),@("windows","arm64"))) {
  $env:GOOS=$t[0]; $env:GOARCH=$t[1]
  go build -o "$env:TEMP\cce-smoke-$($t[0])-$($t[1])" ./cmd/kubectl-cce
  if ($LASTEXITCODE -ne 0) { throw "cross-compile failed for $($t[0])/$($t[1])" }
}
$env:GOOS=$null; $env:GOARCH=$null
Remove-Item "$env:TEMP\cce-smoke-*" -Force -ErrorAction SilentlyContinue
```
Expected: no exception (all 4 targets build).

- [ ] **Step 4: Validate the GoReleaser config**

goreleaser is not installed locally. Validate the config without installing by running it on the fly:

```powershell
go run github.com/goreleaser/goreleaser/v2@latest check
```
Expected: `Configuration is valid` / exit 0 (downloads the goreleaser module first).

- [ ] **Step 5: Commit**

```powershell
git add .goreleaser.yml .gitignore
git commit -m "Add GoReleaser config for multi-platform releases"
```

---

## Task 5: GitHub Actions workflows

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: gofmt
        run: |
          out=$(gofmt -l .)
          if [ -n "$out" ]; then echo "$out"; exit 1; fi
      - name: vet
        run: go vet ./...
      - name: test
        run: go test ./...
      - name: cross-compile
        run: |
          for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
            goos=${target%/*}; goarch=${target#*/}
            GOOS=$goos GOARCH=$goarch go build -o /dev/null ./cmd/kubectl-cce
          done
```

- [ ] **Step 2: Create `.github/workflows/release.yml`**

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 3: Lint the workflow YAML locally (optional but recommended)**

If the YAML is malformed, CI will fail on push. Quick syntax check:

```powershell
go run github.com/goreleaser/goreleaser/v2@latest check
```
(The GoReleaser check does not validate the GitHub Actions YAML, but confirms `.goreleaser.yml` is still valid after the earlier task.) The workflow files themselves are validated by GitHub on push.

- [ ] **Step 4: Commit**

```powershell
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "Add CI and release GitHub Actions workflows"
```

---

## Task 6: Rewrite README install section

**Files:**
- Modify: `README.md` (lines 13-21, the `## Install` section)

- [ ] **Step 1: Replace the Install section**

In `README.md`, find:

```
## Install

```bash
go build -o kubectl-cce ./cmd/kubectl-cce
export PATH="$PWD:$PATH"
kubectl plugin list
```

kubectl discovers the plugin because the executable is named `kubectl-cce`.
```

Replace with:

```
## Install

### From GitHub Releases (recommended)

Download the archive for your platform from
[Releases](https://github.com/pancake0001/kubectl-cce-plugin/releases),
extract it, and put the binary on your `PATH`. Rename it to `kubectl-cce`
(`kubectl-cce.exe` on Windows) so kubectl can discover it.

| OS | Arch | Archive |
| --- | --- | --- |
| Linux | amd64 | `kubectl-cce_<version>_linux_amd64.tar.gz` |
| Linux | arm64 | `kubectl-cce_<version>_linux_arm64.tar.gz` |
| Windows | amd64 | `kubectl-cce_<version>_windows_amd64.zip` |
| Windows | arm64 | `kubectl-cce_<version>_windows_arm64.zip` |

```bash
tar -xzf kubectl-cce_*_linux_amd64.tar.gz
chmod +x kubectl-cce
mv kubectl-cce /usr/local/bin/
kubectl plugin list
kubectl cce --version
```

### From source

```bash
go build -o kubectl-cce ./cmd/kubectl-cce
export PATH="$PWD:$PATH"
kubectl plugin list
```

kubectl discovers the plugin because the executable is named `kubectl-cce`.
```

- [ ] **Step 2: Commit**

```powershell
git add README.md
git commit -m "Rewrite README install section for binary releases"
```

---

## Final verification

After all tasks, run the full local verification suite:

```powershell
gofmt -l .
go vet ./...
go test ./...
```
Expected: gofmt prints nothing; vet and test pass.

Then to publish a release:

```powershell
git tag v0.1.0
git push origin v0.1.0
```
The `release.yml` workflow builds all 4 targets, creates the GitHub Release, and uploads archives + `checksums.txt`. Verify at https://github.com/pancake0001/kubectl-cce-plugin/releases.

## Spec coverage check

- A. version injection + `--version` flag → Task 2 ✅
- B. cross-platform temp kubeconfig → Task 3 ✅
- C. GoReleaser config → Task 4 ✅
- D. GitHub Actions (ci + release) → Task 5 ✅
- E. `.gitattributes` → Task 1 ✅
- F. README install → Task 6 ✅
- ldflags `-X main.version={{.Version}}` → Task 2 (var) + Task 4 (config) ✅
