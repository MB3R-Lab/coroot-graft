package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"coroot-graft/internal/orchestrator"
	"coroot-graft/internal/state"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	orch     *orchestrator.Orchestrator
	addr     string
	logger   *log.Logger
	registry *prometheus.Registry
	triggers map[string]chan string
}

func New(addr string, orch *orchestrator.Orchestrator) *Server {
	registry := prometheus.NewRegistry()
	registry.MustRegister(newMetricsCollector(orch.Store()))
	return &Server{
		orch:     orch,
		addr:     addr,
		logger:   log.New(os.Stdout, "coroot-graft ", log.LstdFlags),
		registry: registry,
		triggers: map[string]chan string{},
	}
}

func (s *Server) Serve(ctx context.Context) error {
	for _, project := range s.orch.Projects() {
		ch := make(chan string, 1)
		s.triggers[project.Name] = ch
		go s.projectLoop(ctx, project.Name, project.Interval, ch)
		if project.Dashboard.Install {
			if id, err := s.orch.InstallDashboard(ctx, project.Name); err != nil {
				s.logger.Printf("dashboard install failed for %s: %v", project.Name, err)
			} else {
				s.logger.Printf("dashboard installed for %s: %s", project.Name, id)
			}
		}
		s.trigger(project.Name, "startup")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.health)
	mux.HandleFunc("/api/v1/projects", s.projects)
	mux.HandleFunc("/api/v1/projects/", s.projectRoutes)
	mux.HandleFunc("/webhooks/coroot/", s.webhook)

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("listening on %s", s.addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/projects" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, s.orch.Store().List())
}

func (s *Server) projectRoutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	project := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		status, ok := s.orch.Store().Get(project)
		if !ok {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, status)
	case len(parts) == 2 && parts[1] == "sync" && r.Method == http.MethodPost:
		if _, ok := s.orch.Project(project); !ok {
			http.Error(w, "project not configured", http.StatusNotFound)
			return
		}
		s.trigger(project, "manual")
		writeJSON(w, http.StatusAccepted, map[string]any{"project": project, "queued": true})
	case len(parts) == 2 && parts[1] == "report" && r.Method == http.MethodGet:
		s.serveFile(w, project, func(status state.ProjectStatus) string { return status.ReportPath }, "application/json")
	case len(parts) == 2 && parts[1] == "summary" && r.Method == http.MethodGet:
		s.serveFile(w, project, func(status state.ProjectStatus) string { return status.SummaryPath }, "text/markdown; charset=utf-8")
	case len(parts) == 2 && parts[1] == "topology" && r.Method == http.MethodGet:
		s.serveFile(w, project, func(status state.ProjectStatus) string { return status.TopologyPath }, "application/yaml")
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project := strings.TrimPrefix(r.URL.Path, "/webhooks/coroot/")
	if project == "" {
		http.NotFound(w, r)
		return
	}
	cfg, ok := s.orch.Project(project)
	if !ok {
		http.Error(w, "project not configured", http.StatusNotFound)
		return
	}
	if cfg.WebhookSecret != "" && r.URL.Query().Get("secret") != cfg.WebhookSecret {
		http.Error(w, "invalid secret", http.StatusUnauthorized)
		return
	}
	s.trigger(project, "coroot-webhook")
	writeJSON(w, http.StatusAccepted, map[string]any{"project": project, "queued": true})
}

func (s *Server) serveFile(w http.ResponseWriter, project string, picker func(state.ProjectStatus) string, contentType string) {
	status, ok := s.orch.Store().Get(project)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	file := picker(status)
	if file == "" {
		http.Error(w, "artifact not available", http.StatusNotFound)
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("read artifact: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(raw)
}

func (s *Server) projectLoop(ctx context.Context, project string, interval time.Duration, ch <-chan string) {
	var ticker *time.Ticker
	if interval > 0 {
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case reason := <-ch:
			if _, err := s.orch.SyncProject(ctx, project, reason); err != nil {
				s.logger.Printf("sync failed for %s: %v", project, err)
			} else {
				s.logger.Printf("sync finished for %s (%s)", project, reason)
			}
		case <-tickerChan(ticker):
			s.trigger(project, "interval")
		}
	}
}

func (s *Server) trigger(project, reason string) {
	ch, ok := s.triggers[project]
	if !ok {
		return
	}
	select {
	case ch <- reason:
	default:
	}
}

func tickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
