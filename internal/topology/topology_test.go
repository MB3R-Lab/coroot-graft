package topology

import (
	"reflect"
	"testing"
	"time"

	"coroot-graft/internal/config"
	"coroot-graft/internal/coroot"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestEvaluateRuntimeActivityUsesBlockingDependencyClosure(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	input := Input{
		Services: []Service{{ID: "frontend"}, {ID: "checkout"}, {ID: "cart"}, {ID: "events"}},
		Edges: []Edge{
			{From: "frontend", To: "checkout", Kind: "sync", Blocking: true},
			{From: "checkout", To: "cart", Kind: "sync", Blocking: true},
			{From: "checkout", To: "events", Kind: "async", Blocking: false},
		},
		Endpoints: []EndpointRef{
			{ID: "view-cart", EntryService: "frontend"},
			{ID: "events-worker", EntryService: "events"},
		},
	}

	impact := EvaluateRuntimeActivity(input, ActivitySnapshot{
		WindowStart: now.Add(-2 * time.Minute),
		WindowEnd:   now,
		ActiveServices: map[string]bool{
			"frontend": true,
			"checkout": true,
			"events":   true,
		},
	})

	if !reflect.DeepEqual(impact.InactiveServices, []string{"cart"}) {
		t.Fatalf("unexpected inactive services: %+v", impact.InactiveServices)
	}
	if !reflect.DeepEqual(impact.ImpactedEndpoints["view-cart"], []string{"cart"}) {
		t.Fatalf("unexpected view-cart impact: %+v", impact.ImpactedEndpoints)
	}
	if _, impacted := impact.ImpactedEndpoints["events-worker"]; impacted {
		t.Fatalf("async dependency must not impact events-worker: %+v", impact.ImpactedEndpoints)
	}
}

func TestBuildUsesSyntheticEndpointsWhenMissing(t *testing.T) {
	in, err := Build(config.ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		EndpointMode:  "service",
	}, Snapshot{
		Project:   "prod",
		CorootRef: "coroot://project/prod/snapshot/2026-03-18T00:00:00Z",
		Apps: []Application{
			{
				ID:       "cluster:default:Deployment:frontend",
				Name:     "frontend",
				Replicas: 2,
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(in.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(in.Endpoints))
	}
	if in.Endpoints[0].ID != "entry::cluster:default:Deployment:frontend" {
		t.Fatalf("unexpected endpoint id: %s", in.Endpoints[0].ID)
	}
}

func TestBuildMarksQueueAsAsync(t *testing.T) {
	in, err := Build(config.ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		EndpointMode:  "service",
	}, Snapshot{
		Project:   "prod",
		CorootRef: "ref",
		Apps: []Application{
			{
				ID:       "a",
				Name:     "a",
				Replicas: 1,
				Dependencies: []Dependency{
					{To: "b", Labels: map[string]string{"queue": "kafka"}},
				},
			},
			{
				ID:       "b",
				Name:     "b",
				Replicas: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(in.Edges) != 1 {
		t.Fatalf("expected one edge, got %d", len(in.Edges))
	}
	if in.Edges[0].Kind != "async" || in.Edges[0].Blocking {
		t.Fatalf("expected async non-blocking edge, got %+v", in.Edges[0])
	}
}

func TestTraceHTTPEndpoints(t *testing.T) {
	got := TraceHTTPEndpoints(coroot.TracingView{
		Spans: []coroot.TracingSpan{
			{Attributes: map[string]string{"http.method": "GET", "http.route": "/checkout"}},
			{Attributes: map[string]string{"http.method": "POST", "http.target": "/orders?id=1"}},
		},
	})
	if len(got) != 2 {
		t.Fatalf("expected two endpoints, got %d", len(got))
	}
	if got[0].Path == "" || got[1].Path == "" {
		t.Fatalf("expected normalized paths, got %+v", got)
	}
}

func TestBuildFiltersApplicationsByIncludeApps(t *testing.T) {
	in, err := Build(config.ProjectConfig{
		Name:              "prod",
		CorootProject:     "prod",
		EndpointMode:      "service",
		IncludeApps:       []string{"demo-*"},
		ExcludeCategories: []string{"monitoring"},
	}, Snapshot{
		Project:   "prod",
		CorootRef: "ref",
		Apps: []Application{
			{ID: "svc:demo-frontend", Name: "demo-frontend", Category: "monitoring", Replicas: 1},
			{ID: "svc:random-sidecar", Name: "random-sidecar", Replicas: 1},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(in.Services) != 1 {
		t.Fatalf("expected one filtered service, got %+v", in.Services)
	}
	if in.Services[0].Name != "demo-frontend" {
		t.Fatalf("unexpected service after filtering: %+v", in.Services[0])
	}
}

func TestBuildAddsMissingEdgeFromOverride(t *testing.T) {
	in, err := Build(config.ProjectConfig{
		Name:          "prod",
		CorootProject: "prod",
		EndpointMode:  "service",
		EdgeOverrides: []config.EdgeOverride{
			{
				From:     "frontend",
				To:       "checkout",
				Kind:     "sync",
				Blocking: boolPtr(true),
			},
		},
	}, Snapshot{
		Project:   "prod",
		CorootRef: "ref",
		Apps: []Application{
			{ID: "frontend", Name: "frontend", Replicas: 1},
			{ID: "checkout", Name: "checkout", Replicas: 1},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(in.Edges) != 1 {
		t.Fatalf("expected one override edge, got %+v", in.Edges)
	}
	if in.Edges[0].From != "frontend" || in.Edges[0].To != "checkout" {
		t.Fatalf("unexpected override edge: %+v", in.Edges[0])
	}
	if in.Edges[0].Kind != "sync" || !in.Edges[0].Blocking {
		t.Fatalf("expected blocking sync edge, got %+v", in.Edges[0])
	}
}
