package topology

import (
	"sort"
	"strings"

	"coroot-graft/internal/coroot"
)

func TraceHTTPEndpoints(view coroot.TracingView) []Endpoint {
	seen := map[string]Endpoint{}
	for _, span := range view.Spans {
		method := strings.ToUpper(strings.TrimSpace(span.Attributes["http.method"]))
		if method == "" {
			continue
		}
		path := extractHTTPPath(span.Attributes)
		if path == "" {
			continue
		}
		id := method + " " + path
		seen[id] = Endpoint{
			ID:     id,
			Method: method,
			Path:   path,
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]Endpoint, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func extractHTTPPath(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	candidates := []string{
		attrs["http.route"],
		attrs["url.path"],
		attrs["http.target"],
		attrs["http.url"],
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		return normalizePath(candidate)
	}
	return ""
}
