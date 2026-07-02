package orchestrator

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coroot-graft/internal/config"
	"coroot-graft/internal/reporting"
	"coroot-graft/internal/toolchain"
)

func TestSyncProjectHappyPathAndDashboardInstall(t *testing.T) {
	mock := newMockCoroot(t)
	defer mock.Close()

	workDir := t.TempDir()
	cfg := testConfig(mock.URL(), workDir)
	cfg.Projects[0].EndpointMode = "trace_http"

	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	orch.tooling = toolchain.NewRunner(helperToolchainConfig(t, "success"))

	status, err := orch.SyncProject(context.Background(), "prod", "test")
	if err != nil {
		t.Fatalf("SyncProject: %v", err)
	}
	if status.Status != "ok" {
		t.Fatalf("expected ok status, got %+v", status)
	}

	rawTopology, err := os.ReadFile(status.TopologyPath)
	if err != nil {
		t.Fatalf("read topology input: %v", err)
	}
	text := string(rawTopology)
	if !strings.Contains(text, "method: GET") || !strings.Contains(text, "path: /checkout") {
		t.Fatalf("expected trace-derived HTTP endpoint in topology input, got:\n%s", text)
	}

	if status.Report == nil || status.Report.PolicyEvaluation.Decision != "pass" {
		t.Fatalf("expected loaded report, got %+v", status.Report)
	}

	id, err := orch.InstallDashboard(context.Background(), "prod")
	if err != nil {
		t.Fatalf("InstallDashboard: %v", err)
	}
	if id == "" {
		t.Fatal("expected dashboard id")
	}
	if len(mock.dashboardSaveBodies) == 0 {
		t.Fatal("expected dashboard save call")
	}
	lastBody := mock.dashboardSaveBodies[len(mock.dashboardSaveBodies)-1]
	if !strings.Contains(lastBody, "coroot_graft_summary_risk_score") {
		t.Fatalf("expected managed dashboard metrics in payload, got %s", lastBody)
	}
}

func TestSyncProjectRetainsPreviousReportOnFailure(t *testing.T) {
	mock := newMockCoroot(t)
	defer mock.Close()

	workDir := t.TempDir()
	cfg := testConfig(mock.URL(), workDir)

	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	orch.tooling = toolchain.NewRunner(helperToolchainConfig(t, "success"))

	first, err := orch.SyncProject(context.Background(), "prod", "first")
	if err != nil {
		t.Fatalf("first SyncProject: %v", err)
	}
	if first.Report == nil {
		t.Fatal("expected first report")
	}

	orch.tooling = toolchain.NewRunner(helperToolchainConfig(t, "fail"))
	second, err := orch.SyncProject(context.Background(), "prod", "second")
	if err == nil {
		t.Fatal("expected failure on second sync")
	}
	if second.Status != "error" {
		t.Fatalf("expected error status, got %+v", second)
	}
	if second.Report == nil {
		t.Fatal("expected previous report to be retained")
	}
	if second.Report.PolicyEvaluation.Decision != first.Report.PolicyEvaluation.Decision {
		t.Fatalf("expected retained report decision %q, got %q", first.Report.PolicyEvaluation.Decision, second.Report.PolicyEvaluation.Decision)
	}
}

func TestSyncProjectFallsBackToOverviewSearchApplications(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "coroot_session", Value: "ok"})
	})
	mux.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"email": "admin",
			"projects": []map[string]any{
				{"id": "prod", "name": "default"},
			},
		})
	})
	mux.HandleFunc("/api/project/prod/overview/map", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
  "context": {
    "search": {
      "applications": [
        {"id": "cluster-a:default:Deployment:frontend"},
        {"id": "cluster-a:default:Deployment:checkout"}
      ]
    }
  },
  "data": {
    "map": null
  }
}`)
	})
	mux.HandleFunc("/api/project/prod/app/", func(w http.ResponseWriter, r *http.Request) {
		appID, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/project/prod/app/"))
		switch appID {
		case "cluster-a:default:Deployment:frontend":
			writeEnvelope(w, map[string]any{
				"app_map": map[string]any{
					"application": map[string]any{
						"id":       appID,
						"category": "product",
						"labels":   map[string]string{"app": "frontend"},
					},
					"instances": []map[string]any{
						{"id": "frontend-1"},
					},
					"dependencies": []map[string]any{
						{
							"id":          "cluster-a:default:Deployment:checkout",
							"category":    "product",
							"labels":      map[string]string{"app": "checkout"},
							"link_stats":  []string{"10 req/s"},
							"link_weight": 10.0,
						},
					},
				},
			})
		case "cluster-a:default:Deployment:checkout":
			writeEnvelope(w, map[string]any{
				"app_map": map[string]any{
					"application": map[string]any{
						"id":       appID,
						"category": "product",
						"labels":   map[string]string{"app": "checkout"},
					},
					"instances": []map[string]any{
						{"id": "checkout-1"},
					},
					"dependencies": []any{},
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	workDir := t.TempDir()
	cfg := testConfig(srv.URL, workDir)
	cfg.Projects[0].CorootProject = "default"

	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	orch.tooling = toolchain.NewRunner(helperToolchainConfig(t, "success"))

	status, err := orch.SyncProject(context.Background(), "prod", "search-fallback")
	if err != nil {
		t.Fatalf("SyncProject: %v", err)
	}
	if status.Status != "ok" {
		t.Fatalf("expected ok status, got %+v", status)
	}

	rawTopology, err := os.ReadFile(status.TopologyPath)
	if err != nil {
		t.Fatalf("read topology input: %v", err)
	}
	text := string(rawTopology)
	for _, want := range []string{
		"cluster-a:default:Deployment:frontend",
		"cluster-a:default:Deployment:checkout",
		"entry::cluster-a:default:Deployment:frontend",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in topology input, got:\n%s", want, text)
		}
	}
}

func TestSyncProjectWithRealBeringAndSheaft(t *testing.T) {
	beringDir := filepath.Clean(filepath.Join("..", "..", ".cache", "upstream", "Bering"))
	sheaftDir := filepath.Clean(filepath.Join("..", "..", ".cache", "upstream", "Sheaft"))
	if _, err := os.Stat(beringDir); err != nil {
		t.Skip("upstream Bering checkout not available")
	}
	if _, err := os.Stat(sheaftDir); err != nil {
		t.Skip("upstream Sheaft checkout not available")
	}

	mock := newMockCoroot(t)
	defer mock.Close()
	workDir := t.TempDir()

	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("mode: warn\ndefault_action: warn\nglobal_threshold: 0.10\nfailure_probability: 0.05\ntrials: 500\n"), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	cfg := testConfig(mock.URL(), workDir)
	cfg.Projects[0].AnalysisPath = ""
	cfg.Projects[0].PolicyPath = policyPath
	cfg.Toolchain.Bering = config.CommandConfig{
		Command:    []string{"go", "run", "./cmd/bering"},
		WorkingDir: beringDir,
	}
	cfg.Toolchain.Sheaft = config.CommandConfig{
		Command:    []string{"go", "run", "./cmd/sheaft"},
		WorkingDir: sheaftDir,
	}

	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, err := orch.SyncProject(context.Background(), "prod", "upstream-e2e")
	if err != nil {
		t.Fatalf("SyncProject with real upstream: %v", err)
	}
	if status.Status != "ok" {
		t.Fatalf("expected ok status, got %+v", status)
	}
	if status.Report == nil {
		t.Fatal("expected report")
	}
	if _, err := os.Stat(status.ModelPath); err != nil {
		t.Fatalf("expected model artifact: %v", err)
	}
	if _, err := os.Stat(status.SnapshotPath); err != nil {
		t.Fatalf("expected snapshot artifact: %v", err)
	}
	if _, err := os.Stat(status.ReportPath); err != nil {
		t.Fatalf("expected report artifact: %v", err)
	}
}

func helperToolchainConfig(t *testing.T, mode string) config.ToolchainConfig {
	t.Helper()
	return config.ToolchainConfig{
		Bering: config.CommandConfig{
			Command: []string{os.Args[0], "-test.run=TestOrchestratorHelperProcess", "--", mode, "bering"},
			Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
		},
		Sheaft: config.CommandConfig{
			Command: []string{os.Args[0], "-test.run=TestOrchestratorHelperProcess", "--", mode, "sheaft"},
			Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
		},
	}
}

func testConfig(baseURL, storageDir string) config.Config {
	return config.Config{
		ListenAddress: ":0",
		StorageDir:    storageDir,
		SyncTimeout:   time.Minute,
		Coroot: config.CorootConfig{
			BaseURL:     baseURL,
			Email:       "admin",
			Password:    "secret",
			HTTPTimeout: time.Second,
			TimeWindow:  time.Hour,
		},
		Toolchain: config.ToolchainConfig{
			Bering: config.CommandConfig{Command: []string{"bering"}},
			Sheaft: config.CommandConfig{Command: []string{"sheaft"}},
		},
		Projects: []config.ProjectConfig{
			{
				Name:          "prod",
				CorootProject: "prod",
				AnalysisPath:  filepath.Join(storageDir, "analysis.yaml"),
				EndpointMode:  "service",
				Dashboard: config.DashboardConfig{
					Install:     true,
					Name:        "Coroot Graft",
					Description: "managed",
				},
				WebhookSecret: "secret",
			},
		},
	}
}

func TestOrchestratorHelperProcess(t *testing.T) {
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
	target := args[1]
	cli := args[2:]

	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "forced helper failure")
		os.Exit(1)
	}

	switch target {
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
		writeHelperJSON(outPath, map[string]any{
			"services": []map[string]any{
				{"id": "cluster-a:default:Deployment:checkout", "name": "checkout", "replicas": 1},
				{"id": "cluster-a:default:Deployment:frontend", "name": "frontend", "replicas": 2},
			},
			"edges": []map[string]any{
				{"from": "cluster-a:default:Deployment:frontend", "to": "cluster-a:default:Deployment:checkout", "kind": "sync", "blocking": true},
			},
			"endpoints": []map[string]any{
				{"id": "cluster-a:default:Deployment:frontend:GET /checkout", "entry_service": "cluster-a:default:Deployment:frontend", "success_predicate_ref": "cluster-a:default:Deployment:frontend:GET /checkout"},
			},
			"metadata": map[string]any{
				"source_type":   "bering",
				"source_ref":    "helper://bering",
				"discovered_at": "2026-03-18T10:00:00Z",
				"confidence":    1.0,
				"schema": map[string]any{
					"name":    "io.mb3r.bering.model",
					"version": "1.3.0",
					"uri":     "https://mb3r-lab.github.io/Bering/schema/model/v1.3.0/model.schema.json",
					"digest":  "sha256:2aa8a3550a25dc626ba6d2f5833569efca2f382b9e5c9c3405be93695d7d48ae",
				},
			},
		})
		writeHelperJSON(snapshotPath, map[string]any{
			"snapshot_id":      "snapshot-1",
			"topology_version": "v1",
			"window_start":     "2026-03-18T09:00:00Z",
			"window_end":       "2026-03-18T10:00:00Z",
			"ingest":           map[string]any{"spans": 0, "traces": 0, "dropped_spans": 0, "late_spans": 0},
			"counts":           map[string]any{"services": 2, "edges": 1, "endpoints": 1},
			"coverage":         map[string]any{"confidence": 1.0, "service_support_min": 1, "edge_support_min": 1, "endpoint_support_min": 1},
			"sources":          []map[string]any{{"type": "topology_api", "ref": "helper://topology"}},
			"diff":             map[string]any{"added_services": 0, "removed_services": 0, "changed_services": 0, "added_edges": 0, "removed_edges": 0, "changed_edges": 0, "added_endpoints": 0, "removed_endpoints": 0, "changed_endpoints": 0},
			"discovery":        map[string]any{"services": []any{}, "edges": []any{}, "endpoints": []any{}},
			"model": map[string]any{
				"services": []map[string]any{
					{"id": "cluster-a:default:Deployment:checkout", "name": "checkout", "replicas": 1},
					{"id": "cluster-a:default:Deployment:frontend", "name": "frontend", "replicas": 2},
				},
				"edges": []map[string]any{
					{"from": "cluster-a:default:Deployment:frontend", "to": "cluster-a:default:Deployment:checkout", "kind": "sync", "blocking": true},
				},
				"endpoints": []map[string]any{
					{"id": "cluster-a:default:Deployment:frontend:GET /checkout", "entry_service": "cluster-a:default:Deployment:frontend", "success_predicate_ref": "cluster-a:default:Deployment:frontend:GET /checkout"},
				},
				"metadata": map[string]any{
					"source_type":   "bering",
					"source_ref":    "helper://bering",
					"discovered_at": "2026-03-18T10:00:00Z",
					"confidence":    1.0,
					"schema": map[string]any{
						"name":    "io.mb3r.bering.model",
						"version": "1.3.0",
						"uri":     "https://mb3r-lab.github.io/Bering/schema/model/v1.3.0/model.schema.json",
						"digest":  "sha256:2aa8a3550a25dc626ba6d2f5833569efca2f382b9e5c9c3405be93695d7d48ae",
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
					"version": "1.3.0",
					"uri":     "https://mb3r-lab.github.io/Bering/schema/snapshot/v1.3.0/snapshot.schema.json",
					"digest":  "sha256:cb778e5b0866d9ce5cfe7f23b8d98a339603593a0247cccd9cddaf05c7ae4bb1",
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
		rep := reporting.Report{
			Simulation: reporting.Simulation{Trials: 1000, Seed: 42, FailureProbability: 0.05},
			EndpointResults: []reporting.EndpointResult{
				{Profile: "steady-state", EndpointID: "cluster-a:default:Deployment:frontend:GET /checkout", Availability: 0.99, Threshold: 0.95, Status: "pass"},
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
					Name: "steady-state",
					Simulation: reporting.ProfileSimulation{
						Name:                 "steady-state",
						Trials:               1000,
						Seed:                 42,
						SamplingMode:         "independent_replica",
						FailureProbability:   0.05,
						EndpointAvailability: map[string]float64{"cluster-a:default:Deployment:frontend:GET /checkout": 0.99},
						WeightedAggregate:    0.99,
						UnweightedAggregate:  0.99,
					},
					EndpointResults: []reporting.EndpointResult{
						{Profile: "steady-state", EndpointID: "cluster-a:default:Deployment:frontend:GET /checkout", Availability: 0.99, Threshold: 0.95, Status: "pass"},
					},
					Decision: "pass",
				},
			},
		}
		writeHelperJSON(filepath.Join(outDir, "report.json"), rep)
		writeHelperJSON(filepath.Join(outDir, "model.json"), map[string]any{"ok": true})
		_ = os.WriteFile(filepath.Join(outDir, "summary.md"), []byte("# ok\n"), 0o644)
	default:
		os.Exit(2)
	}

	os.Exit(0)
}

type mockCoroot struct {
	server              *httptest.Server
	dashboardSaveBodies []string
	dashboardID         string
}

func newMockCoroot(t *testing.T) *mockCoroot {
	t.Helper()
	mock := &mockCoroot{dashboardID: "dash-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "coroot_session", Value: "ok"})
	})
	mux.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"email": "admin",
			"projects": []map[string]any{
				{"id": "prod", "name": "default"},
			},
		})
	})
	mux.HandleFunc("/api/project/prod/overview/map", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, map[string]any{
			"map": []map[string]any{
				{
					"id":       "cluster-a:default:Deployment:frontend",
					"cluster":  "cluster-a",
					"category": "product",
					"labels":   map[string]string{"app": "frontend"},
					"status":   "ok",
				},
				{
					"id":       "cluster-a:default:Deployment:checkout",
					"cluster":  "cluster-a",
					"category": "product",
					"labels":   map[string]string{"app": "checkout"},
					"status":   "ok",
				},
			},
		})
	})
	mux.HandleFunc("/api/project/prod/app/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/project/prod/app/")
		if strings.HasSuffix(rest, "/tracing") {
			appID, _ := url.PathUnescape(strings.TrimSuffix(rest, "/tracing"))
			switch appID {
			case "cluster-a:default:Deployment:frontend":
				writeEnvelope(w, map[string]any{
					"spans": []map[string]any{
						{"attributes": map[string]string{"http.method": "GET", "http.route": "/checkout"}},
					},
				})
			default:
				writeEnvelope(w, map[string]any{"spans": []any{}})
			}
			return
		}

		appID, _ := url.PathUnescape(rest)
		switch appID {
		case "cluster-a:default:Deployment:frontend":
			writeEnvelope(w, map[string]any{
				"app_map": map[string]any{
					"application": map[string]any{"id": appID},
					"instances": []map[string]any{
						{"id": "frontend-1"},
						{"id": "frontend-2"},
					},
					"clients": []any{},
					"dependencies": []map[string]any{
						{
							"id":          "cluster-a:default:Deployment:checkout",
							"category":    "product",
							"labels":      map[string]string{"app": "checkout"},
							"link_stats":  []string{"12 req/s"},
							"link_weight": 12.0,
						},
					},
				},
			})
		case "cluster-a:default:Deployment:checkout":
			writeEnvelope(w, map[string]any{
				"app_map": map[string]any{
					"application":  map[string]any{"id": appID},
					"instances":    []map[string]any{{"id": "checkout-1"}},
					"clients":      []any{},
					"dependencies": []any{},
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	mux.HandleFunc("/api/project/prod/dashboards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeEnvelope(w, []map[string]any{})
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(mock.dashboardID))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/project/prod/dashboards/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mock.dashboardSaveBodies = append(mock.dashboardSaveBodies, string(body))
		w.WriteHeader(http.StatusOK)
	})

	mock.server = httptest.NewServer(mux)
	return mock
}

func (m *mockCoroot) URL() string {
	return m.server.URL
}

func (m *mockCoroot) Close() {
	m.server.Close()
}

func writeEnvelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": data,
	})
}

func writeHelperJSON(path string, value any) {
	raw, _ := json.MarshalIndent(value, "", "  ")
	_ = os.WriteFile(path, raw, 0o644)
}
