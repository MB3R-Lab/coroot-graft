package topology

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"coroot-graft/internal/config"
)

type Snapshot struct {
	Project    string
	CorootRef  string
	CapturedAt time.Time
	Apps       []Application
	Activity   ActivitySnapshot
}

type ActivitySnapshot struct {
	WindowStart    time.Time       `json:"window_start"`
	WindowEnd      time.Time       `json:"window_end"`
	ActiveServices map[string]bool `json:"-"`
}

type RuntimeImpact struct {
	WindowStart       time.Time           `json:"window_start"`
	WindowEnd         time.Time           `json:"window_end"`
	ActiveServices    []string            `json:"active_services"`
	InactiveServices  []string            `json:"inactive_services"`
	ImpactedEndpoints map[string][]string `json:"impacted_endpoints"`
}

type Application struct {
	ID           string
	Name         string
	Category     string
	Labels       map[string]string
	Replicas     int
	Dependencies []Dependency
	Endpoints    []Endpoint
}

type Dependency struct {
	To     string
	Labels map[string]string
	Stats  []string
	Weight float64
}

type Endpoint struct {
	ID     string
	Method string
	Path   string
}

type Input struct {
	Source    Source        `yaml:"source" json:"source"`
	Services  []Service     `yaml:"services" json:"services"`
	Edges     []Edge        `yaml:"edges" json:"edges"`
	Endpoints []EndpointRef `yaml:"endpoints" json:"endpoints"`
}

type Source struct {
	Type string `yaml:"type" json:"type"`
	Ref  string `yaml:"ref" json:"ref"`
}

type Service struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Replicas int    `yaml:"replicas,omitempty" json:"replicas,omitempty"`
}

type Edge struct {
	From     string  `yaml:"from" json:"from"`
	To       string  `yaml:"to" json:"to"`
	Kind     string  `yaml:"kind" json:"kind"`
	Blocking bool    `yaml:"blocking" json:"blocking"`
	Weight   float64 `yaml:"weight,omitempty" json:"weight,omitempty"`
}

type EndpointRef struct {
	ID           string `yaml:"id,omitempty" json:"id,omitempty"`
	EntryService string `yaml:"entry_service" json:"entry_service"`
	Method       string `yaml:"method,omitempty" json:"method,omitempty"`
	Path         string `yaml:"path,omitempty" json:"path,omitempty"`
	PredicateRef string `yaml:"predicate_ref,omitempty" json:"predicate_ref,omitempty"`
}

func Build(project config.ProjectConfig, snapshot Snapshot) (Input, error) {
	apps := filterApplications(project, snapshot.Apps)
	if len(apps) == 0 {
		return Input{}, fmt.Errorf("snapshot for project %s produced no applications after filtering", snapshot.Project)
	}

	serviceSet := map[string]Application{}
	for _, app := range apps {
		serviceSet[app.ID] = app
	}

	services := make([]Service, 0, len(apps))
	edgesByKey := map[string]Edge{}
	endpointsByID := map[string]EndpointRef{}

	for _, app := range apps {
		replicas := app.Replicas
		if replicas <= 0 {
			replicas = 1
		}
		services = append(services, Service{
			ID:       app.ID,
			Name:     app.Name,
			Replicas: replicas,
		})

		for _, dep := range app.Dependencies {
			if _, ok := serviceSet[dep.To]; !ok {
				continue
			}
			kind, blocking := inferEdge(project, app.ID, dep)
			key := app.ID + "->" + dep.To
			edgesByKey[key] = Edge{
				From:     app.ID,
				To:       dep.To,
				Kind:     kind,
				Blocking: blocking,
				Weight:   dep.Weight,
			}
		}

		endpoints := app.Endpoints
		if len(endpoints) == 0 {
			endpoints = []Endpoint{syntheticEndpoint(app.ID)}
		}
		for _, ep := range endpoints {
			ref := EndpointRef{
				EntryService: app.ID,
				PredicateRef: endpointPredicateRef(ep),
			}
			if ep.Method != "" && ep.Path != "" {
				ref.Method = strings.ToUpper(strings.TrimSpace(ep.Method))
				ref.Path = normalizePath(ep.Path)
			} else {
				ref.ID = ep.ID
			}
			if ref.ID == "" && ref.Method == "" {
				ref.ID = ep.ID
			}
			id := endpointStableID(ref, app.ID)
			if ref.ID == "" {
				ref.ID = id
			}
			endpointsByID[id] = ref
		}
	}

	applyEdgeOverrides(project, serviceSet, edgesByKey)

	sort.Slice(services, func(i, j int) bool {
		return services[i].ID < services[j].ID
	})

	edges := make([]Edge, 0, len(edgesByKey))
	for _, edge := range edgesByKey {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	endpoints := make([]EndpointRef, 0, len(endpointsByID))
	ids := make([]string, 0, len(endpointsByID))
	for id := range endpointsByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		endpoints = append(endpoints, endpointsByID[id])
	}

	return Input{
		Source: Source{
			Type: "topology_api",
			Ref:  snapshot.CorootRef,
		},
		Services:  services,
		Edges:     edges,
		Endpoints: endpoints,
	}, nil
}

// EvaluateRuntimeActivity keeps the stable topology intact while determining
// which endpoints are unavailable in the current Coroot observation window.
// An endpoint is unavailable when its entry service or a transitively required
// blocking synchronous dependency was not observed.
func EvaluateRuntimeActivity(input Input, activity ActivitySnapshot) RuntimeImpact {
	impact := RuntimeImpact{
		WindowStart:       activity.WindowStart,
		WindowEnd:         activity.WindowEnd,
		ImpactedEndpoints: map[string][]string{},
	}

	services := make(map[string]struct{}, len(input.Services))
	for _, service := range input.Services {
		services[service.ID] = struct{}{}
		if activity.ActiveServices[service.ID] {
			impact.ActiveServices = append(impact.ActiveServices, service.ID)
		} else {
			impact.InactiveServices = append(impact.InactiveServices, service.ID)
		}
	}
	sort.Strings(impact.ActiveServices)
	sort.Strings(impact.InactiveServices)

	required := map[string][]string{}
	for _, edge := range input.Edges {
		if !edge.Blocking || edge.Kind == "async" {
			continue
		}
		required[edge.From] = append(required[edge.From], edge.To)
	}

	for _, endpoint := range input.Endpoints {
		endpointID := endpoint.ID
		if endpointID == "" {
			endpointID = endpointStableID(endpoint, endpoint.EntryService)
		}
		seen := map[string]bool{}
		stack := []string{endpoint.EntryService}
		missing := make([]string, 0)
		for len(stack) > 0 {
			last := len(stack) - 1
			serviceID := stack[last]
			stack = stack[:last]
			if seen[serviceID] {
				continue
			}
			seen[serviceID] = true
			if _, known := services[serviceID]; known && !activity.ActiveServices[serviceID] {
				missing = append(missing, serviceID)
			}
			stack = append(stack, required[serviceID]...)
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			impact.ImpactedEndpoints[endpointID] = missing
		}
	}

	return impact
}

func filterApplications(project config.ProjectConfig, apps []Application) []Application {
	if len(apps) == 0 {
		return nil
	}
	excluded := map[string]struct{}{}
	for _, category := range project.ExcludeCategories {
		excluded[strings.ToLower(category)] = struct{}{}
	}

	filtered := make([]Application, 0, len(apps))
	for _, app := range apps {
		if app.ID == "" {
			continue
		}
		included := len(project.IncludeApps) > 0 && matchesIncludeApp(project.IncludeApps, app)
		if len(project.IncludeApps) > 0 && !included {
			continue
		}
		if !included {
			if !project.IncludeExternal && isExternal(app.ID) {
				continue
			}
			if _, ok := excluded[strings.ToLower(app.Category)]; ok {
				continue
			}
		}
		filtered = append(filtered, app)
	}
	return filtered
}

func matchesIncludeApp(patterns []string, app Application) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, app.ID) || globMatch(pattern, app.Name) {
			return true
		}
	}
	return false
}

func globMatch(pattern, value string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" || value == "" {
		return false
	}
	if ok, err := pathMatch(pattern, value); err == nil && ok {
		return true
	}
	return strings.Contains(value, strings.Trim(pattern, "*"))
}

func pathMatch(pattern, value string) (bool, error) {
	return path.Match(pattern, value)
}

func inferEdge(project config.ProjectConfig, from string, dep Dependency) (string, bool) {
	kind := "sync"
	blocking := true
	for _, override := range project.EdgeOverrides {
		if override.From != from || override.To != dep.To {
			continue
		}
		if override.Kind != "" {
			kind = override.Kind
		}
		if override.Blocking != nil {
			blocking = *override.Blocking
		}
		return kind, blocking
	}

	if isAsyncByLabels(dep.Labels) || isAsyncByIdentifier(dep.To) {
		return "async", false
	}
	return kind, blocking
}

func applyEdgeOverrides(project config.ProjectConfig, services map[string]Application, edgesByKey map[string]Edge) {
	for _, override := range project.EdgeOverrides {
		if _, ok := services[override.From]; !ok {
			continue
		}
		if _, ok := services[override.To]; !ok {
			continue
		}

		key := override.From + "->" + override.To
		edge, exists := edgesByKey[key]
		if !exists {
			edge = Edge{
				From:     override.From,
				To:       override.To,
				Kind:     "sync",
				Blocking: true,
			}
		}
		if override.Kind != "" {
			edge.Kind = override.Kind
		}
		if !exists && override.Kind == "async" && override.Blocking == nil {
			edge.Blocking = false
		}
		if override.Blocking != nil {
			edge.Blocking = *override.Blocking
		}
		edgesByKey[key] = edge
	}
}

func syntheticEndpoint(serviceID string) Endpoint {
	return Endpoint{
		ID: "entry::" + serviceID,
	}
}

func endpointPredicateRef(ep Endpoint) string {
	if ep.ID != "" {
		return ep.ID
	}
	return strings.ToUpper(strings.TrimSpace(ep.Method)) + " " + normalizePath(ep.Path)
}

func endpointStableID(ref EndpointRef, serviceID string) string {
	if ref.ID != "" {
		return ref.ID
	}
	if ref.Method != "" && ref.Path != "" {
		return serviceID + ":" + ref.Method + " " + ref.Path
	}
	return "entry::" + serviceID
}

func normalizePath(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "/"
	}
	switch {
	case strings.HasPrefix(v, "http://"), strings.HasPrefix(v, "https://"):
		if parsed, err := url.Parse(v); err == nil && parsed.Path != "" {
			v = parsed.Path
		}
	}
	if idx := strings.Index(v, "?"); idx >= 0 {
		v = v[:idx]
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	return v
}

func isExternal(id string) bool {
	return strings.HasPrefix(id, "external:")
}

func isAsyncByLabels(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	if _, ok := labels["queue"]; ok {
		return true
	}
	for _, value := range labels {
		if isAsyncByIdentifier(value) {
			return true
		}
	}
	return false
}

func isAsyncByIdentifier(value string) bool {
	v := strings.ToLower(value)
	return strings.Contains(v, "kafka") ||
		strings.Contains(v, "rabbitmq") ||
		strings.Contains(v, "nats")
}
