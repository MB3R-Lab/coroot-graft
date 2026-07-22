package state

import (
	"sort"
	"sync"
	"time"

	"coroot-graft/internal/reporting"
)

type ProjectStatus struct {
	Project        string            `json:"project"`
	Trigger        string            `json:"trigger"`
	Status         string            `json:"status"`
	Error          string            `json:"error,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     time.Time         `json:"finished_at"`
	RunDir         string            `json:"run_dir,omitempty"`
	TopologyPath   string            `json:"topology_path,omitempty"`
	ModelPath      string            `json:"model_path,omitempty"`
	SnapshotPath   string            `json:"snapshot_path,omitempty"`
	ReportPath     string            `json:"report_path,omitempty"`
	SummaryPath    string            `json:"summary_path,omitempty"`
	RawReportPath  string            `json:"raw_sheaft_report_path,omitempty"`
	RawSummaryPath string            `json:"raw_sheaft_summary_path,omitempty"`
	ActivityPath   string            `json:"runtime_activity_path,omitempty"`
	Report         *reporting.Report `json:"report,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	projects map[string]ProjectStatus
}

func New() *Store {
	return &Store{
		projects: map[string]ProjectStatus{},
	}
}

func (s *Store) Put(status ProjectStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[status.Project] = status
}

func (s *Store) Get(project string) (ProjectStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, ok := s.projects[project]
	return status, ok
}

func (s *Store) List() []ProjectStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProjectStatus, 0, len(s.projects))
	for _, status := range s.projects {
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Project < out[j].Project
	})
	return out
}
