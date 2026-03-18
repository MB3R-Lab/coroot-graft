package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectValidationRejectsMixedAnalysisAndPolicy(t *testing.T) {
	p := ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		AnalysisPath:  "a.yaml",
		PolicyPath:    "p.yaml",
		EndpointMode:  "service",
	}
	if err := p.Validate(0); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestProjectValidationAllowsAnalysisOnly(t *testing.T) {
	p := ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		AnalysisPath:  "a.yaml",
		EndpointMode:  "service",
	}
	if err := p.Validate(0); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestLoadExpandsEnvironmentVariables(t *testing.T) {
	t.Setenv("COROOT_GRAFT_TEST_PASSWORD", "secret-from-env")

	root := t.TempDir()
	configPath := filepath.Join(root, "graft.yaml")
	analysisPath := filepath.Join(root, "analysis.yaml")
	if err := os.WriteFile(analysisPath, []byte("schema_version: \"1.0\"\n"), 0o644); err != nil {
		t.Fatalf("write analysis: %v", err)
	}

	raw := `
listen_address: ":8095"
storage_dir: ".coroot-graft"
sync_timeout: 10m

coroot:
  base_url: "http://127.0.0.1:8080"
  email: "admin"
  password: "${COROOT_GRAFT_TEST_PASSWORD}"
  http_timeout: 30s
  time_window: 1h

toolchain:
  bering:
    command: ["bering"]
  sheaft:
    command: ["sheaft"]

projects:
  - name: "prod"
    coroot_project: "prod"
    endpoint_mode: "service"
    analysis: "./analysis.yaml"
`
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Coroot.Password != "secret-from-env" {
		t.Fatalf("expected env-expanded password, got %q", cfg.Coroot.Password)
	}
	if cfg.Projects[0].AnalysisPath != analysisPath {
		t.Fatalf("expected normalized analysis path %q, got %q", analysisPath, cfg.Projects[0].AnalysisPath)
	}
}

func TestProjectValidationRejectsEmptyIncludeAppEntry(t *testing.T) {
	p := ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		AnalysisPath:  "a.yaml",
		EndpointMode:  "service",
		IncludeApps:   []string{"demo-*", "   "},
	}
	if err := p.Validate(0); err == nil {
		t.Fatal("expected validation error")
	}
}
