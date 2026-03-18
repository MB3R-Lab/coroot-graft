package coroot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientReloginsOnUnauthorized(t *testing.T) {
	loginCalls := 0
	protectedCalls := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		loginCalls++
		http.SetCookie(w, &http.Cookie{Name: "coroot_session", Value: "ok"})
	})
	mux.HandleFunc("/api/project/prod/overview/map", func(w http.ResponseWriter, r *http.Request) {
		protectedCalls++
		_, err := r.Cookie("coroot_session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(envelope[OverviewView]{Data: OverviewView{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := NewClient(srv.URL, "admin", "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.GetOverviewMap(context.Background(), "prod", time.Now(), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("GetOverviewMap: %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("expected one login call, got %d", loginCalls)
	}
	if protectedCalls != 2 {
		t.Fatalf("expected two protected calls, got %d", protectedCalls)
	}
}

func TestGetOverviewMapIncludesSearchApplications(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "coroot_session", Value: "ok"})
	})
	mux.HandleFunc("/api/project/prod/overview/map", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "context": {
    "search": {
      "applications": [
        {"id": "prod:_:Unknown:frontend"},
        {"id": "prod:_:Unknown:checkout"}
      ]
    }
  },
  "data": {
    "map": null
  }
}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := NewClient(srv.URL, "admin", "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	view, err := client.GetOverviewMap(context.Background(), "prod", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("GetOverviewMap: %v", err)
	}
	if len(view.Map) != 0 {
		t.Fatalf("expected empty map payload, got %+v", view.Map)
	}
	if len(view.SearchApplications) != 2 {
		t.Fatalf("expected two search applications, got %+v", view.SearchApplications)
	}
	if view.SearchApplications[0].ID != "prod:_:Unknown:frontend" {
		t.Fatalf("unexpected first search application: %+v", view.SearchApplications[0])
	}
}

func TestResolveProjectByName(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "coroot_session", Value: "ok"})
	})
	mux.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "email": "admin",
  "projects": [
    {"id": "pbe6k51m", "name": "default"},
    {"id": "prod-id", "name": "production"}
  ]
}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := NewClient(srv.URL, "admin", "secret", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	project, err := client.ResolveProject(context.Background(), "default")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if project.ID != "pbe6k51m" {
		t.Fatalf("unexpected project id: %+v", project)
	}
}
