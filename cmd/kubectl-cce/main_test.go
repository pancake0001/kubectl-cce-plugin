package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalURIAddsTrailingSlash(t *testing.T) {
	got := canonicalURI("/v1/project-id/vpcs")
	want := "/v1/project-id/vpcs/"
	if got != want {
		t.Fatalf("canonicalURI() = %q, want %q", got, want)
	}
}

func TestAKSKSignedHeadersMatchHuaweiGoSDKShape(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://service.region.example.com/v1/project-id/vpcs?limit=2&marker=abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "service.region.example.com"
	req.Header.Set("Host", "service.region.example.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sdk-Date", "20191115T033655Z")

	signedHeaders := signedHeaders(req.Header)
	canonicalHeaders := canonicalHeaders(req, signedHeaders)
	if signedHeaders != "host;x-sdk-date" {
		t.Fatalf("signedHeaders = %q", signedHeaders)
	}
	wantHeaders := "host:service.region.example.com\nx-sdk-date:20191115T033655Z\n"
	if canonicalHeaders != wantHeaders {
		t.Fatalf("canonicalHeaders = %q, want %q", canonicalHeaders, wantHeaders)
	}
}

func TestCleanEnvValueRemovesSmartQuotes(t *testing.T) {
	got := cleanEnvValue(" ‘6feaa266-79e4-11f1-946d-0255ac100260’ ")
	want := "6feaa266-79e4-11f1-946d-0255ac100260"
	if got != want {
		t.Fatalf("cleanEnvValue() = %q, want %q", got, want)
	}
}

func TestApplyCLIOverrides(t *testing.T) {
	cfg := config{
		clusterID: "env-cluster",
		region:    "cn-north-4",
		endpoint:  "",
		projectID: "env-project",
	}
	cfg.applyCLIOverrides(" cli-cluster ", "cn-east-3", " ‘example.com’ ", "cli-project")

	if cfg.clusterID != "cli-cluster" {
		t.Fatalf("clusterID = %q", cfg.clusterID)
	}
	if cfg.region != "cn-east-3" {
		t.Fatalf("region = %q", cfg.region)
	}
	if cfg.endpoint != "example.com" {
		t.Fatalf("endpoint = %q", cfg.endpoint)
	}
	if cfg.projectID != "cli-project" {
		t.Fatalf("projectID = %q", cfg.projectID)
	}
}

func TestExecIsBlocked(t *testing.T) {
	if command := findUnsupportedStreamingCommand([]string{"exec", "pod/nginx", "--", "date"}); command != "exec" {
		t.Fatalf("findUnsupportedStreamingCommand() = %q, want exec", command)
	}
}

func TestCanonicalQuerySortsKeysAndValues(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/path?b=2&a=z&a=x", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := canonicalQuery(req.URL.Query())
	want := "a=x&a=z&b=2"
	if got != want {
		t.Fatalf("canonicalQuery() = %q, want %q", got, want)
	}
}

func TestVersionFlagPrintsAndExits(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--version"}, &stdout); err != nil {
		t.Fatalf("run(--version) err = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}
}

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

func TestEnvDefaultAliases(t *testing.T) {
	t.Setenv("HW_REGION", "")
	t.Setenv("HUAWEICLOUD_REGION", "")
	t.Setenv("HUAWEI_CLOUD_REGION", "")
	if got := envDefaultAliases("cn-north-4", "HW_REGION", "HUAWEICLOUD_REGION", "HUAWEI_CLOUD_REGION"); got != "cn-north-4" {
		t.Fatalf("fallback got %q, want cn-north-4", got)
	}
	t.Setenv("HW_REGION", "cn-east-3")
	if got := envDefaultAliases("cn-north-4", "HW_REGION", "HUAWEICLOUD_REGION", "HUAWEI_CLOUD_REGION"); got != "cn-east-3" {
		t.Fatalf("first key got %q, want cn-east-3", got)
	}
	t.Setenv("HW_REGION", "")
	t.Setenv("HUAWEI_CLOUD_REGION", "cn-south-1")
	if got := envDefaultAliases("cn-north-4", "HW_REGION", "HUAWEICLOUD_REGION", "HUAWEI_CLOUD_REGION"); got != "cn-south-1" {
		t.Fatalf("alias got %q, want cn-south-1", got)
	}
}

func TestParseArgsExtractsCLICredsAnywhere(t *testing.T) {
	parsed, err := parseArgs([]string{"get", "pods", "-n", "default", "--cli-access-key", "AK", "--cli-secret-key", "SK"})
	if err != nil {
		t.Fatalf("parseArgs err = %v", err)
	}
	if got := parsed.values["cli-access-key"]; got != "AK" {
		t.Fatalf("cli-access-key = %q, want AK", got)
	}
	if got := parsed.values["cli-secret-key"]; got != "SK" {
		t.Fatalf("cli-secret-key = %q, want SK", got)
	}
	want := []string{"get", "pods", "-n", "default"}
	if !reflect.DeepEqual(parsed.kubectlArgs, want) {
		t.Fatalf("kubectlArgs = %v, want %v", parsed.kubectlArgs, want)
	}
}

func TestParseArgsHandlesEqualsForm(t *testing.T) {
	parsed, err := parseArgs([]string{"--cli-access-key=AK", "--cli-secret-key=SK", "get", "pods", "--namespace=foo"})
	if err != nil {
		t.Fatalf("parseArgs err = %v", err)
	}
	if got := parsed.values["cli-access-key"]; got != "AK" {
		t.Fatalf("cli-access-key = %q, want AK", got)
	}
	if got := parsed.values["cli-secret-key"]; got != "SK" {
		t.Fatalf("cli-secret-key = %q, want SK", got)
	}
	want := []string{"get", "pods", "--namespace=foo"}
	if !reflect.DeepEqual(parsed.kubectlArgs, want) {
		t.Fatalf("kubectlArgs = %v, want %v", parsed.kubectlArgs, want)
	}
}

func TestParseArgsBoolFlagsPositionIndependent(t *testing.T) {
	parsed, err := parseArgs([]string{"get", "pods", "--print-proxy-url"})
	if err != nil {
		t.Fatalf("parseArgs err = %v", err)
	}
	if !parsed.bools["print-proxy-url"] {
		t.Fatal("print-proxy-url not set")
	}
	want := []string{"get", "pods"}
	if !reflect.DeepEqual(parsed.kubectlArgs, want) {
		t.Fatalf("kubectlArgs = %v, want %v", parsed.kubectlArgs, want)
	}
}

func TestParseArgsMissingValue(t *testing.T) {
	if _, err := parseArgs([]string{"get", "--cli-access-key"}); err == nil {
		t.Fatal("expected error for missing value, got nil")
	}
}

func TestValidateRequiresProjectID(t *testing.T) {
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

	cfg.projectID = "project-y"
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() err = %v, want nil when project id is set", err)
	}
}

func TestProxyForwardsProjectIdInIAMPath(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")

	var gotProjectID string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProjectID = r.Header.Get("X-Project-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer upstream.Close()

	cfg := config{
		endpoint:    strings.TrimPrefix(upstream.URL, "https://"),
		projectID:   "proj-iam-123",
		iamToken:    "iam-token-abc",
		insecureTLS: true,
	}
	proxySrv := httptest.NewServer(proxyHandler(cfg))
	defer proxySrv.Close()

	resp, err := http.Get(proxySrv.URL + "/api")
	if err != nil {
		t.Fatalf("http get through proxy: %v", err)
	}
	defer resp.Body.Close()

	if gotProjectID != "proj-iam-123" {
		t.Fatalf("upstream X-Project-Id = %q, want proj-iam-123", gotProjectID)
	}
}
