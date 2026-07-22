package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coroot-graft/internal/config"
	"coroot-graft/internal/orchestrator"
	"coroot-graft/internal/reporting"
	"coroot-graft/internal/state"
	"coroot-graft/internal/topology"

	io_prometheus_client "github.com/prometheus/client_model/go"
)

func TestProjectRoutesServeStoredStatusAndArtifacts(t *testing.T) {
	srv, trigger, status := newTestServer(t)

	projectsRec := httptest.NewRecorder()
	srv.projects(projectsRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	if projectsRec.Code != http.StatusOK {
		t.Fatalf("projects status: %d", projectsRec.Code)
	}
	var list []state.ProjectStatus
	if err := json.NewDecoder(projectsRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode projects response: %v", err)
	}
	if len(list) != 1 || list[0].Project != "prod" {
		t.Fatalf("unexpected projects list: %+v", list)
	}

	statusRec := httptest.NewRecorder()
	srv.projectRoutes(statusRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod", nil))
	if statusRec.Code != http.StatusOK {
		t.Fatalf("project status code: %d", statusRec.Code)
	}
	var got state.ProjectStatus
	if err := json.NewDecoder(statusRec.Body).Decode(&got); err != nil {
		t.Fatalf("decode project response: %v", err)
	}
	if got.Project != status.Project || got.Report == nil || got.Report.PolicyEvaluation.Decision != "pass" {
		t.Fatalf("unexpected project payload: %+v", got)
	}

	reportRec := httptest.NewRecorder()
	srv.projectRoutes(reportRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod/report", nil))
	if reportRec.Code != http.StatusOK {
		t.Fatalf("report status code: %d", reportRec.Code)
	}
	if body := reportRec.Body.String(); !strings.Contains(body, `"decision":"pass"`) || !strings.Contains(body, `"endpoint_id":"entry::frontend"`) {
		t.Fatalf("unexpected report body: %q", body)
	}

	rawReportRec := httptest.NewRecorder()
	srv.projectRoutes(rawReportRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod/sheaft-report", nil))
	if rawReportRec.Code != http.StatusOK || !strings.Contains(rawReportRec.Body.String(), `"raw":true`) {
		t.Fatalf("unexpected raw Sheaft report response: code=%d body=%q", rawReportRec.Code, rawReportRec.Body.String())
	}

	activityRec := httptest.NewRecorder()
	srv.projectRoutes(activityRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod/activity", nil))
	if activityRec.Code != http.StatusOK || !strings.Contains(activityRec.Body.String(), `"inactive_services":["cart"]`) {
		t.Fatalf("unexpected activity response: code=%d body=%q", activityRec.Code, activityRec.Body.String())
	}

	summaryRec := httptest.NewRecorder()
	srv.projectRoutes(summaryRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod/summary", nil))
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("summary status code: %d", summaryRec.Code)
	}
	if summaryRec.Body.String() != "# summary\n" {
		t.Fatalf("unexpected summary body: %q", summaryRec.Body.String())
	}

	topologyRec := httptest.NewRecorder()
	srv.projectRoutes(topologyRec, httptest.NewRequest(http.MethodGet, "/api/v1/projects/prod/topology", nil))
	if topologyRec.Code != http.StatusOK {
		t.Fatalf("topology status code: %d", topologyRec.Code)
	}
	if topologyRec.Body.String() != "source:\n  type: topology_api\n" {
		t.Fatalf("unexpected topology body: %q", topologyRec.Body.String())
	}

	syncRec := httptest.NewRecorder()
	srv.projectRoutes(syncRec, httptest.NewRequest(http.MethodPost, "/api/v1/projects/prod/sync", nil))
	if syncRec.Code != http.StatusAccepted {
		t.Fatalf("sync status code: %d", syncRec.Code)
	}
	select {
	case reason := <-trigger:
		if reason != "manual" {
			t.Fatalf("unexpected trigger reason: %q", reason)
		}
	default:
		t.Fatal("expected manual trigger to be queued")
	}
}

func TestWebhookRequiresSecretAndQueuesSync(t *testing.T) {
	srv, trigger, _ := newTestServer(t)

	badRec := httptest.NewRecorder()
	srv.webhook(badRec, httptest.NewRequest(http.MethodPost, "/webhooks/coroot/prod?secret=wrong", nil))
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", badRec.Code)
	}

	okRec := httptest.NewRecorder()
	srv.webhook(okRec, httptest.NewRequest(http.MethodPost, "/webhooks/coroot/prod?secret=secret", nil))
	if okRec.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d", okRec.Code)
	}
	select {
	case reason := <-trigger:
		if reason != "coroot-webhook" {
			t.Fatalf("unexpected trigger reason: %q", reason)
		}
	default:
		t.Fatal("expected webhook trigger to be queued")
	}
}

func TestMetricsCollectorExportsLatestReportState(t *testing.T) {
	srv, _, _ := newTestServer(t)

	families, err := srv.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	assertGaugeValue(t, families, "coroot_graft_last_run_success", map[string]string{"project": "prod"}, 1)
	assertGaugeValue(t, families, "coroot_graft_gate_decision_state", map[string]string{"project": "prod", "decision": "pass"}, 1)
	assertGaugeValue(t, families, "coroot_graft_summary_risk_score", map[string]string{"project": "prod"}, 0.01)
	assertGaugeValue(t, families, "coroot_graft_contract_policy_state", map[string]string{"project": "prod", "status": "current", "action": "warn"}, 1)
	assertGaugeValue(t, families, "coroot_graft_profile_decision_state", map[string]string{"project": "prod", "profile": "steady-state", "decision": "pass"}, 1)
	assertGaugeValue(t, families, "coroot_graft_endpoint_status_state", map[string]string{"project": "prod", "profile": "steady-state", "endpoint": "entry::frontend", "status": "pass"}, 1)
	assertGaugeValue(t, families, "coroot_graft_service_observed", map[string]string{"project": "prod", "service": "frontend"}, 1)
	assertGaugeValue(t, families, "coroot_graft_service_observed", map[string]string{"project": "prod", "service": "cart"}, 0)
	assertGaugeValue(t, families, "coroot_graft_endpoint_runtime_available", map[string]string{"project": "prod", "endpoint": "entry::frontend"}, 0)
}

func newTestServer(t *testing.T) (*Server, chan string, state.ProjectStatus) {
	t.Helper()

	workDir := t.TempDir()
	cfg := config.Config{
		ListenAddress: ":0",
		StorageDir:    workDir,
		SyncTimeout:   time.Minute,
		Coroot: config.CorootConfig{
			BaseURL:     "http://127.0.0.1",
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
				AnalysisPath:  filepath.Join(workDir, "analysis.yaml"),
				WebhookSecret: "secret",
			},
		},
	}

	orch, err := orchestrator.New(cfg)
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}

	report := reporting.Report{
		PolicyEvaluation: reporting.PolicyEvaluation{
			Mode:            "warn",
			Decision:        "pass",
			FailedEndpoints: []string{},
		},
		Summary: reporting.Summary{
			RiskScore:  0.01,
			Confidence: 1,
		},
		EndpointResults: []reporting.EndpointResult{
			{Profile: "steady-state", EndpointID: "entry::frontend", Availability: 0.99, Threshold: 0.95, Status: "pass"},
		},
		Profiles: []reporting.Profile{
			{
				Name: "steady-state",
				Simulation: reporting.ProfileSimulation{
					Name:                 "steady-state",
					Trials:               1000,
					Seed:                 42,
					SamplingMode:         "independent_replica",
					EndpointAvailability: map[string]float64{"entry::frontend": 0.99},
					WeightedAggregate:    0.99,
					UnweightedAggregate:  0.99,
				},
				EndpointResults: []reporting.EndpointResult{
					{Profile: "steady-state", EndpointID: "entry::frontend", Availability: 0.99, Threshold: 0.95, Status: "pass"},
				},
				Decision: "pass",
			},
		},
		ContractPolicy: &reporting.ContractPolicy{
			Status: "current",
			Action: "warn",
		},
		RuntimeActivity: &topology.RuntimeImpact{
			ActiveServices:    []string{"frontend"},
			InactiveServices:  []string{"cart"},
			ImpactedEndpoints: map[string][]string{"entry::frontend": {"cart"}},
		},
	}

	reportPath := filepath.Join(workDir, "report.json")
	rawReport, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(reportPath, append(rawReport, '\n'), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	rawSheaftReportPath := filepath.Join(workDir, "sheaft-report.json")
	if err := os.WriteFile(rawSheaftReportPath, []byte("{\"raw\":true}\n"), 0o644); err != nil {
		t.Fatalf("write raw Sheaft report: %v", err)
	}
	activityPath := filepath.Join(workDir, "runtime-activity.json")
	if err := os.WriteFile(activityPath, []byte("{\"inactive_services\":[\"cart\"]}\n"), 0o644); err != nil {
		t.Fatalf("write runtime activity: %v", err)
	}

	summaryPath := filepath.Join(workDir, "summary.md")
	if err := os.WriteFile(summaryPath, []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	topologyPath := filepath.Join(workDir, "topology.yaml")
	if err := os.WriteFile(topologyPath, []byte("source:\n  type: topology_api\n"), 0o644); err != nil {
		t.Fatalf("write topology: %v", err)
	}

	status := state.ProjectStatus{
		Project:       "prod",
		Trigger:       "test",
		Status:        "ok",
		StartedAt:     time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 3, 18, 10, 0, 2, 0, time.UTC),
		RunDir:        workDir,
		TopologyPath:  topologyPath,
		ReportPath:    reportPath,
		SummaryPath:   summaryPath,
		RawReportPath: rawSheaftReportPath,
		ActivityPath:  activityPath,
		Report:        &report,
	}
	orch.Store().Put(status)

	srv := New(":0", orch)
	trigger := make(chan string, 2)
	srv.triggers["prod"] = trigger
	return srv, trigger, status
}

func assertGaugeValue(t *testing.T, families []*io_prometheus_client.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if hasLabels(metric, labels) {
				if metric.GetGauge().GetValue() != want {
					t.Fatalf("metric %s labels %v = %v, want %v", name, labels, metric.GetGauge().GetValue(), want)
				}
				return
			}
		}
		t.Fatalf("metric %s with labels %v not found", name, labels)
	}
	t.Fatalf("metric family %s not found", name)
}

func hasLabels(metric *io_prometheus_client.Metric, labels map[string]string) bool {
	if len(metric.GetLabel()) != len(labels) {
		return false
	}
	for _, label := range metric.GetLabel() {
		if labels[label.GetName()] != label.GetValue() {
			return false
		}
	}
	return true
}
