package orchestrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coroot-graft/internal/config"
	"coroot-graft/internal/coroot"
	"coroot-graft/internal/reporting"
	"coroot-graft/internal/state"
	"coroot-graft/internal/toolchain"
	"coroot-graft/internal/topology"

	"gopkg.in/yaml.v3"
)

type Orchestrator struct {
	cfg      config.Config
	coroot   *coroot.Client
	tooling  *toolchain.Runner
	store    *state.Store
	projects map[string]config.ProjectConfig
}

func New(cfg config.Config) (*Orchestrator, error) {
	client, err := coroot.NewClient(cfg.Coroot.BaseURL, cfg.Coroot.Email, cfg.Coroot.Password, cfg.Coroot.HTTPTimeout)
	if err != nil {
		return nil, err
	}
	projects := make(map[string]config.ProjectConfig, len(cfg.Projects))
	for _, project := range cfg.Projects {
		projects[project.Name] = project
	}
	return &Orchestrator{
		cfg:      cfg,
		coroot:   client,
		tooling:  toolchain.NewRunner(cfg.Toolchain),
		store:    state.New(),
		projects: projects,
	}, nil
}

func (o *Orchestrator) Store() *state.Store {
	return o.store
}

func (o *Orchestrator) Projects() []config.ProjectConfig {
	out := make([]config.ProjectConfig, 0, len(o.cfg.Projects))
	out = append(out, o.cfg.Projects...)
	return out
}

func (o *Orchestrator) Project(name string) (config.ProjectConfig, bool) {
	project, ok := o.projects[name]
	return project, ok
}

func (o *Orchestrator) SyncAll(ctx context.Context, trigger string) error {
	for _, project := range o.cfg.Projects {
		if _, err := o.SyncProject(ctx, project.Name, trigger); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) SyncProject(ctx context.Context, projectName, trigger string) (state.ProjectStatus, error) {
	project, ok := o.projects[projectName]
	if !ok {
		return state.ProjectStatus{}, fmt.Errorf("unknown project %q", projectName)
	}
	previous, hasPrevious := o.store.Get(projectName)
	restorePrevious := func(status *state.ProjectStatus) {
		if !hasPrevious || previous.Report == nil {
			return
		}
		if status.TopologyPath == "" {
			status.TopologyPath = previous.TopologyPath
		}
		if status.ModelPath == "" {
			status.ModelPath = previous.ModelPath
		}
		if status.SnapshotPath == "" {
			status.SnapshotPath = previous.SnapshotPath
		}
		if status.ReportPath == "" {
			status.ReportPath = previous.ReportPath
		}
		if status.SummaryPath == "" {
			status.SummaryPath = previous.SummaryPath
		}
		if status.RunDir == "" {
			status.RunDir = previous.RunDir
		}
		status.Report = previous.Report
	}

	startedAt := time.Now().UTC()
	status := state.ProjectStatus{
		Project:   projectName,
		Trigger:   trigger,
		Status:    "running",
		StartedAt: startedAt,
	}
	o.store.Put(status)

	ctx, cancel := context.WithTimeout(ctx, o.cfg.SyncTimeout)
	defer cancel()

	resolvedProject, err := o.resolveProject(ctx, project)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	snapshot, err := o.captureSnapshot(ctx, resolvedProject, startedAt)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	projectRoot := filepath.Join(o.cfg.StorageDir, "projects", project.Name)
	runDir := filepath.Join(projectRoot, "runs", startedAt.Format("20060102T150405Z"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		status.Status = "error"
		status.Error = fmt.Sprintf("create run dir: %v", err)
		status.FinishedAt = time.Now().UTC()
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	input, err := topology.Build(resolvedProject, snapshot)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	topologyPath := filepath.Join(runDir, "topology-input.yaml")
	if err := writeYAML(topologyPath, input); err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	runResult, err := o.tooling.Run(ctx, toolchain.Request{
		Project:      resolvedProject,
		TopologyPath: topologyPath,
		RunDir:       runDir,
		DiscoveredAt: startedAt,
	})
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		status.RunDir = runDir
		status.TopologyPath = topologyPath
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	report, err := reporting.Load(runResult.ReportPath)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		status.RunDir = runDir
		status.TopologyPath = topologyPath
		status.ModelPath = runResult.ModelPath
		status.SnapshotPath = runResult.SnapshotPath
		status.ReportPath = runResult.ReportPath
		status.SummaryPath = runResult.SummaryPath
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	if err := o.refreshLatest(projectRoot, topologyPath, runResult); err != nil {
		status.Status = "error"
		status.Error = err.Error()
		status.FinishedAt = time.Now().UTC()
		status.RunDir = runDir
		status.TopologyPath = topologyPath
		status.ModelPath = runResult.ModelPath
		status.SnapshotPath = runResult.SnapshotPath
		status.ReportPath = runResult.ReportPath
		status.SummaryPath = runResult.SummaryPath
		restorePrevious(&status)
		o.store.Put(status)
		return status, err
	}

	status = state.ProjectStatus{
		Project:      projectName,
		Trigger:      trigger,
		Status:       "ok",
		StartedAt:    startedAt,
		FinishedAt:   time.Now().UTC(),
		RunDir:       filepath.Join(projectRoot, "latest"),
		TopologyPath: filepath.Join(projectRoot, "latest", "topology-input.yaml"),
		ModelPath:    filepath.Join(projectRoot, "latest", "bering-model.json"),
		SnapshotPath: filepath.Join(projectRoot, "latest", "bering-snapshot.json"),
		ReportPath:   filepath.Join(projectRoot, "latest", "report.json"),
		SummaryPath:  filepath.Join(projectRoot, "latest", "summary.md"),
		Report:       report,
	}
	o.store.Put(status)
	return status, nil
}

func (o *Orchestrator) captureSnapshot(ctx context.Context, project config.ProjectConfig, capturedAt time.Time) (topology.Snapshot, error) {
	window := o.cfg.Coroot.TimeWindow
	if project.TimeWindow > 0 {
		window = project.TimeWindow
	}
	from := capturedAt.Add(-window)
	to := capturedAt

	overview, err := o.coroot.GetOverviewMap(ctx, project.CorootProject, from, to)
	if err != nil {
		return topology.Snapshot{}, err
	}

	overviewApps := append([]coroot.OverviewApplication(nil), overview.Map...)
	if len(overviewApps) == 0 && len(overview.SearchApplications) > 0 {
		overviewApps = make([]coroot.OverviewApplication, 0, len(overview.SearchApplications))
		for _, app := range overview.SearchApplications {
			overviewApps = append(overviewApps, coroot.OverviewApplication{ID: app.ID})
		}
	}

	apps := make([]topology.Application, 0, len(overviewApps))
	for _, app := range overviewApps {
		view, err := o.coroot.GetApplication(ctx, project.CorootProject, app.ID, from, to)
		if err != nil {
			return topology.Snapshot{}, fmt.Errorf("load application %s: %w", app.ID, err)
		}

		category := app.Category
		if category == "" {
			category = view.AppMap.Application.Category
		}
		labels := cloneLabels(app.Labels)
		if len(labels) == 0 {
			labels = cloneLabels(view.AppMap.Application.Labels)
		}

		item := topology.Application{
			ID:       app.ID,
			Name:     appName(app.ID),
			Category: category,
			Labels:   labels,
			Replicas: len(view.AppMap.Instances),
		}
		for _, dep := range view.AppMap.Dependencies {
			item.Dependencies = append(item.Dependencies, topology.Dependency{
				To:     dep.ID,
				Labels: cloneLabels(dep.Labels),
				Stats:  append([]string(nil), dep.LinkStats...),
				Weight: dep.LinkWeight,
			})
		}

		if project.EndpointMode == "trace_http" {
			traceView, traceErr := o.coroot.GetTracing(ctx, project.CorootProject, app.ID, from, to)
			if traceErr == nil {
				item.Endpoints = topology.TraceHTTPEndpoints(traceView)
			}
		}

		apps = append(apps, item)
	}

	return topology.Snapshot{
		Project:    project.Name,
		CorootRef:  corootRef(project.CorootProject, capturedAt),
		CapturedAt: capturedAt,
		Apps:       apps,
	}, nil
}

func (o *Orchestrator) resolveProject(ctx context.Context, project config.ProjectConfig) (config.ProjectConfig, error) {
	resolved, err := o.coroot.ResolveProject(ctx, project.CorootProject)
	if err != nil {
		return config.ProjectConfig{}, err
	}
	project.CorootProject = resolved.ID
	return project, nil
}

func (o *Orchestrator) refreshLatest(projectRoot, topologyPath string, result toolchain.Result) error {
	latestDir := filepath.Join(projectRoot, "latest")
	if err := os.MkdirAll(latestDir, 0o755); err != nil {
		return fmt.Errorf("create latest dir: %w", err)
	}
	files := map[string]string{
		topologyPath:        filepath.Join(latestDir, "topology-input.yaml"),
		result.ModelPath:    filepath.Join(latestDir, "bering-model.json"),
		result.SnapshotPath: filepath.Join(latestDir, "bering-snapshot.json"),
		result.ReportPath:   filepath.Join(latestDir, "report.json"),
		result.SummaryPath:  filepath.Join(latestDir, "summary.md"),
	}
	for src, dst := range files {
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func writeYAML(path string, value any) error {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode topology yaml: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write topology yaml: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func appName(id string) string {
	parts := strings.SplitN(id, ":", 4)
	if len(parts) == 4 {
		return parts[3]
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return id
}

func corootRef(project string, capturedAt time.Time) string {
	return fmt.Sprintf("coroot://project/%s/snapshot/%s", url.PathEscape(project), capturedAt.UTC().Format(time.RFC3339))
}

func cloneLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
