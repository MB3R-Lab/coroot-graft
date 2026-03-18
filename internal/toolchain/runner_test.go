package toolchain

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coroot-graft/internal/config"
	"coroot-graft/internal/reporting"
)

func TestRunnerUsesAnalysisMode(t *testing.T) {
	runner := NewRunner(config.ToolchainConfig{
		Bering: helperCommandConfig(t, "bering"),
		Sheaft: helperCommandConfig(t, "sheaft"),
	})

	runDir := t.TempDir()
	result, err := runner.Run(context.Background(), Request{
		Project: config.ProjectConfig{
			Name:         "prod",
			AnalysisPath: filepath.Join(runDir, "analysis.yaml"),
		},
		TopologyPath: filepath.Join(runDir, "topology.yaml"),
		RunDir:       runDir,
		DiscoveredAt: time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(result.ModelPath); err != nil {
		t.Fatalf("expected model path: %v", err)
	}
	if _, err := os.Stat(result.SnapshotPath); err != nil {
		t.Fatalf("expected snapshot path: %v", err)
	}
	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatalf("expected report path: %v", err)
	}

	argsPath := filepath.Join(runDir, "helper-sheaft-args.txt")
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read helper args: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "--analysis") {
		t.Fatalf("expected --analysis in args, got %q", got)
	}
	if strings.Contains(got, "--policy") {
		t.Fatalf("did not expect --policy in args, got %q", got)
	}
}

func TestRunnerUsesPolicyJourneysAndContractPolicy(t *testing.T) {
	runner := NewRunner(config.ToolchainConfig{
		Bering: helperCommandConfig(t, "bering"),
		Sheaft: helperCommandConfig(t, "sheaft"),
	})

	runDir := t.TempDir()
	result, err := runner.Run(context.Background(), Request{
		Project: config.ProjectConfig{
			Name:               "prod",
			PolicyPath:         filepath.Join(runDir, "policy.yaml"),
			ContractPolicyPath: filepath.Join(runDir, "contracts.yaml"),
			JourneysPath:       filepath.Join(runDir, "journeys.yaml"),
		},
		TopologyPath: filepath.Join(runDir, "topology.yaml"),
		RunDir:       runDir,
		DiscoveredAt: time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatalf("expected report path: %v", err)
	}

	argsPath := filepath.Join(runDir, "helper-sheaft-args.txt")
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read helper args: %v", err)
	}
	got := string(raw)
	for _, want := range []string{
		"--policy",
		filepath.Join(runDir, "policy.yaml"),
		"--contract-policy",
		filepath.Join(runDir, "contracts.yaml"),
		"--journeys",
		filepath.Join(runDir, "journeys.yaml"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in args, got %q", want, got)
		}
	}
	if strings.Contains(got, "--analysis") {
		t.Fatalf("did not expect --analysis in args, got %q", got)
	}
}

func helperCommandConfig(t *testing.T, mode string) config.CommandConfig {
	t.Helper()
	return config.CommandConfig{
		Command: []string{os.Args[0], "-test.run=TestRunnerHelperProcess", "--", mode},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
		},
	}
}

func TestRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	flag.Parse()
	args := os.Args
	for i, arg := range args {
		if arg != "--" {
			continue
		}
		args = args[i+1:]
		break
	}
	if len(args) < 2 {
		os.Exit(2)
	}

	mode := args[0]
	cli := args[1:]
	switch mode {
	case "bering":
		var outPath, snapshotPath string
		for i := 0; i < len(cli); i++ {
			switch cli[i] {
			case "--out":
				outPath = cli[i+1]
			case "--snapshot-out":
				snapshotPath = cli[i+1]
			}
		}
		writeJSONFile(outPath, map[string]any{
			"services": []map[string]any{{"id": "frontend", "name": "frontend", "replicas": 2}},
			"edges":    []map[string]any{},
			"endpoints": []map[string]any{
				{"id": "entry::frontend", "entry_service": "frontend", "success_predicate_ref": "entry::frontend"},
			},
			"metadata": map[string]any{
				"source_type":   "bering",
				"source_ref":    "helper://bering",
				"discovered_at": "2026-03-18T10:00:00Z",
				"confidence":    1.0,
				"schema": map[string]any{
					"name":    "io.mb3r.bering.model",
					"version": "1.0.0",
					"uri":     "https://example.invalid/model.schema.json",
					"digest":  "sha256:test",
				},
			},
		})
		writeJSONFile(snapshotPath, map[string]any{
			"snapshot_id":      "snapshot-1",
			"topology_version": "v1",
			"window_start":     "2026-03-18T09:00:00Z",
			"window_end":       "2026-03-18T10:00:00Z",
			"ingest":           map[string]any{"spans": 0, "traces": 0, "dropped_spans": 0, "late_spans": 0},
			"counts":           map[string]any{"services": 1, "edges": 0, "endpoints": 1},
			"coverage":         map[string]any{"confidence": 1.0, "service_support_min": 1, "edge_support_min": 1, "endpoint_support_min": 1},
			"sources":          []map[string]any{{"type": "topology_api", "ref": "helper://topology"}},
			"diff":             map[string]any{"added_services": 0, "removed_services": 0, "changed_services": 0, "added_edges": 0, "removed_edges": 0, "changed_edges": 0, "added_endpoints": 0, "removed_endpoints": 0, "changed_endpoints": 0},
			"discovery":        map[string]any{"services": []any{}, "edges": []any{}, "endpoints": []any{}},
			"model": map[string]any{
				"services": []map[string]any{{"id": "frontend", "name": "frontend", "replicas": 2}},
				"edges":    []map[string]any{},
				"endpoints": []map[string]any{
					{"id": "entry::frontend", "entry_service": "frontend", "success_predicate_ref": "entry::frontend"},
				},
				"metadata": map[string]any{
					"source_type":   "bering",
					"source_ref":    "helper://bering",
					"discovered_at": "2026-03-18T10:00:00Z",
					"confidence":    1.0,
					"schema": map[string]any{
						"name":    "io.mb3r.bering.model",
						"version": "1.0.0",
						"uri":     "https://example.invalid/model.schema.json",
						"digest":  "sha256:test",
					},
				},
			},
			"metadata": map[string]any{
				"source_type": "bering",
				"source_ref":  "helper://bering",
				"emitted_at":  "2026-03-18T10:00:00Z",
				"confidence":  1.0,
				"schema": map[string]any{
					"name":    "io.mb3r.bering.snapshot",
					"version": "1.0.0",
					"uri":     "https://example.invalid/snapshot.schema.json",
					"digest":  "sha256:test-snapshot",
				},
			},
		})
	case "sheaft":
		var outDir string
		for i := 0; i < len(cli); i++ {
			if cli[i] == "--out-dir" {
				outDir = cli[i+1]
			}
		}
		_ = os.MkdirAll(outDir, 0o755)
		argsPath := filepath.Join(filepath.Dir(outDir), "helper-sheaft-args.txt")
		_ = os.WriteFile(argsPath, []byte(strings.Join(cli, " ")), 0o644)
		rep := reporting.Report{
			Simulation: reporting.Simulation{Trials: 1000, Seed: 42, FailureProbability: 0.05},
			EndpointResults: []reporting.EndpointResult{
				{Profile: "default", EndpointID: "entry::frontend", Availability: 0.99, Threshold: 0.95, Status: "pass"},
			},
			Summary: reporting.Summary{
				OverallAvailability:              0.99,
				WeightedOverallAvailability:      0.99,
				CrossProfileAvailability:         0.99,
				CrossProfileWeightedAvailability: 0.99,
				RiskScore:                        0.01,
				Confidence:                       1,
			},
			PolicyEvaluation: reporting.PolicyEvaluation{
				Mode:            "warn",
				Decision:        "pass",
				FailedEndpoints: []string{},
			},
			Profiles: []reporting.Profile{
				{
					Name: "default",
					Simulation: reporting.ProfileSimulation{
						Name:                 "default",
						Trials:               1000,
						Seed:                 42,
						SamplingMode:         "independent_replica",
						FailureProbability:   0.05,
						EndpointAvailability: map[string]float64{"entry::frontend": 0.99},
						WeightedAggregate:    0.99,
						UnweightedAggregate:  0.99,
					},
					EndpointResults: []reporting.EndpointResult{
						{Profile: "default", EndpointID: "entry::frontend", Availability: 0.99, Threshold: 0.95, Status: "pass"},
					},
					Decision: "pass",
				},
			},
		}
		writeJSONFile(filepath.Join(outDir, "report.json"), rep)
		writeJSONFile(filepath.Join(outDir, "model.json"), map[string]any{"ok": true})
		_ = os.WriteFile(filepath.Join(outDir, "summary.md"), []byte("# ok\n"), 0o644)
	default:
		os.Exit(2)
	}

	os.Exit(0)
}

func writeJSONFile(path string, value any) {
	raw, _ := json.MarshalIndent(value, "", "  ")
	_ = os.WriteFile(path, raw, 0o644)
}
