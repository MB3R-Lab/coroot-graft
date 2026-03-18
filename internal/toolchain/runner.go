package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"coroot-graft/internal/config"
)

type Runner struct {
	bering config.CommandConfig
	sheaft config.CommandConfig
}

type Request struct {
	Project      config.ProjectConfig
	TopologyPath string
	RunDir       string
	DiscoveredAt time.Time
}

type Result struct {
	RunDir       string
	ModelPath    string
	SnapshotPath string
	ReportPath   string
	SummaryPath  string
}

func NewRunner(toolchain config.ToolchainConfig) *Runner {
	return &Runner{
		bering: toolchain.Bering,
		sheaft: toolchain.Sheaft,
	}
}

func (r *Runner) Run(ctx context.Context, req Request) (Result, error) {
	if err := os.MkdirAll(req.RunDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create run dir: %w", err)
	}

	modelPath := filepath.Join(req.RunDir, "bering-model.json")
	snapshotPath := filepath.Join(req.RunDir, "bering-snapshot.json")
	sheaftDir := filepath.Join(req.RunDir, "sheaft")
	if err := os.MkdirAll(sheaftDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create sheaft output dir: %w", err)
	}

	beringArgs := []string{
		"discover",
		"--input", req.TopologyPath,
		"--out", modelPath,
		"--snapshot-out", snapshotPath,
		"--discovered-at", req.DiscoveredAt.UTC().Format(time.RFC3339),
	}
	if req.Project.OverlayPath != "" {
		beringArgs = append(beringArgs, "--overlay", req.Project.OverlayPath)
	}
	if _, err := r.exec(ctx, r.bering, beringArgs...); err != nil {
		return Result{}, err
	}

	sheaftArgs := []string{
		"run",
		"--model", snapshotPath,
	}
	switch {
	case req.Project.AnalysisPath != "":
		sheaftArgs = append(sheaftArgs, "--analysis", req.Project.AnalysisPath)
	case req.Project.PolicyPath != "":
		sheaftArgs = append(sheaftArgs, "--policy", req.Project.PolicyPath)
	default:
		return Result{}, fmt.Errorf("project %s has neither analysis nor policy configured", req.Project.Name)
	}
	if req.Project.ContractPolicyPath != "" {
		sheaftArgs = append(sheaftArgs, "--contract-policy", req.Project.ContractPolicyPath)
	}
	if req.Project.PolicyPath != "" && req.Project.JourneysPath != "" {
		sheaftArgs = append(sheaftArgs, "--journeys", req.Project.JourneysPath)
	}
	sheaftArgs = append(sheaftArgs, "--out-dir", sheaftDir)
	if _, err := r.exec(ctx, r.sheaft, sheaftArgs...); err != nil {
		return Result{}, err
	}

	return Result{
		RunDir:       req.RunDir,
		ModelPath:    modelPath,
		SnapshotPath: snapshotPath,
		ReportPath:   filepath.Join(sheaftDir, "report.json"),
		SummaryPath:  filepath.Join(sheaftDir, "summary.md"),
	}, nil
}

func (r *Runner) exec(ctx context.Context, cfg config.CommandConfig, args ...string) ([]byte, error) {
	cmdArgs := append([]string{}, cfg.Command[1:]...)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, cfg.Command[0], cmdArgs...)
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}
	cmd.Env = os.Environ()
	for key, value := range cfg.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s failed: %w\n%s", strings.Join(append([]string{cfg.Command[0]}, args...), " "), err, strings.TrimSpace(output.String()))
	}
	return output.Bytes(), nil
}
