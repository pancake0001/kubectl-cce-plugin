# Require project_id Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Huawei Cloud project id a required input (env var or `--project-id` CLI flag) so that `X-Project-Id` is always present on upstream CCE requests, and ship release v0.3.0.

**Architecture:** Add a required-field check in `config.validate()` that fires for every auth path, with an error message that names `--project-id` and `HW_PROJECT_ID` so an executing skill can read the stderr and supply the flag. Also set `X-Project-Id` in the IAM-token fallback branch of `proxyHandler` so the header is sent regardless of auth method. No new files; all changes in `cmd/kubectl-cce/main.go`, `cmd/kubectl-cce/main_test.go`, and `README.md`.

**Tech Stack:** Go 1.25 (stdlib only), stdlib `testing`, GoReleaser for release.

---

## Task 1: Failing test for required projectID

**Files:**
- Modify: `cmd/kubectl-cce/main_test.go` (append new test)

- [ ] **Step 1: Write the failing test**

Append to `cmd/kubectl-cce/main_test.go`:

```go
func TestValidateRequiresProjectID(t *testing.T) {
	// Missing project id -> error that names --project-id (so a skill reading
	// stderr can perceive it and pass the flag).
	cfg := config{
		clusterID: "cluster-x",
		ak:        "AK",
		sk:        "SK",
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("validate() err = nil, want error for missing project id")
	}
	if !strings.Contains(err.Error(), "--project-id") {
		t.Fatalf("validate() err = %q, want error mentioning --project-id", err)
	}

	// Present project id -> no error from this check (auth satisfied).
	cfg.projectID = "project-y"
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() err = %v, want nil when project id is set", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/kubectl-cce/ -run TestValidateRequiresProjectID -v`
Expected: FAIL — `validate() err = nil, want error for missing project id` (current `validate()` does not check `projectID`).

## Task 2: Require projectID in validate()

**Files:**
- Modify: `cmd/kubectl-cce/main.go:205-216` (the `validate` func)

- [ ] **Step 3: Write minimal implementation**

Replace the body of `func (c config) validate() error` with:

```go
func (c config) validate() error {
	if c.projectID == "" {
		return errors.New("project id is required: pass --project-id, or set HW_PROJECT_ID (HUAWEICLOUD_PROJECT_ID / HUAWEI_CLOUD_PROJECT_ID are also accepted)")
	}
	if c.endpoint == "" && c.clusterID == "" {
		return errors.New("CCE_CLUSTER_ID is required unless CCE_ENDPOINT is set")
	}
	if c.ak != "" && c.sk != "" {
		return nil
	}
	if c.iamToken != "" {
		return nil
	}
	return errors.New("set --cli-access-key and --cli-secret-key (or HW_ACCESS_KEY/HW_SECRET_KEY), or set HUAWEI_IAM_TOKEN")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/kubectl-cce/ -run TestValidateRequiresProjectID -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./...`
Expected: PASS (all existing tests still pass — none reach `validate()` except via `--version` which returns early).

## Task 3: Send X-Project-Id in the IAM-token path

**Files:**
- Modify: `cmd/kubectl-cce/main.go:337-342` (the auth branch in `proxyHandler`)

- [ ] **Step 6: Write minimal implementation**

Change the `else` branch so the IAM-token path also forwards the header:

```go
		if cfg.useAKSK() {
			applyAKSKSignature(req, body, cfg)
		} else {
			req.Header.Set("X-Auth-Token", cfg.iamToken)
			if cfg.projectID != "" {
				req.Header.Set("X-Project-Id", cfg.projectID)
			}
			req.Header.Del("Authorization")
		}
```

(Note: `projectID` is now guaranteed non-empty after `validate()`, but the guard is kept defensive and cheap.)

- [ ] **Step 7: Run full test suite + vet + fmt**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: tests PASS, vet clean, gofmt prints nothing.

## Task 4: Document --project-id in README

**Files:**
- Modify: `README.md:77` and surrounding Configure section

- [ ] **Step 8: Update README**

Change the `HW_PROJECT_ID` line from "optional, but recommended for AK/SK" to required, and add a `--project-id` CLI example. The project id is required for all auth methods.

## Task 5: Verify and build locally

- [ ] **Step 9: Build with release ldflags**

Run:
```
CGO_ENABLED=0 go build -trimpath -buildmode=pie -ldflags="-s -w -X main.version=0.3.0" -o kubectl-cce.exe ./cmd/kubectl-cce
.\kubectl-cce.exe --version
```
Expected: prints `0.3.0`.

- [ ] **Step 10: Confirm missing project id surfaces the actionable error**

Run (with `HW_PROJECT_ID` and `HUAWEI_*_PROJECT_ID` unset):
```
$env:HW_PROJECT_ID=$null; $env:HUAWEICLOUD_PROJECT_ID=$null; $env:HUAWEI_CLOUD_PROJECT_ID=$null; .\kubectl-cce.exe --cluster-id c --cli-access-key a --cli-secret-key b get ns
```
Expected: stderr line `kubectl-cce: project id is required: pass --project-id, ...` and exit code 2.

## Task 6: Commit, tag, release

- [ ] **Step 11: Commit**

```
git add cmd/kubectl-cce/main.go cmd/kubectl-cce/main_test.go README.md docs/superpowers/plans/2026-09-17-require-project-id.md
git commit -m "feat: require project id and forward X-Project-Id for all auth paths"
```

- [ ] **Step 12: Tag and push**

```
git tag v0.3.0
git push origin v0.3.0
```
Expected: GoReleaser release workflow runs on GitHub Actions; release `0.3.0` published with linux/windows × amd64/arm64 archives + `checksums.txt`. (Gitee handled separately by user.)
