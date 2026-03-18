package coroot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	baseURL  *url.URL
	email    string
	password string
	http     *http.Client
}

type UserProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OverviewView struct {
	Map                []OverviewApplication    `json:"map"`
	SearchApplications []OverviewApplicationRef `json:"-"`
}

type OverviewApplication struct {
	ID          string            `json:"id"`
	Cluster     string            `json:"cluster"`
	Category    string            `json:"category"`
	Custom      bool              `json:"custom"`
	Labels      map[string]string `json:"labels"`
	Status      string            `json:"status"`
	Upstreams   []OverviewLink    `json:"upstreams"`
	Downstreams []OverviewLink    `json:"downstreams"`
}

type OverviewApplicationRef struct {
	ID string `json:"id"`
}

type OverviewLink struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Stats  []string `json:"stats"`
	Weight float64  `json:"weight"`
}

type ApplicationView struct {
	AppMap ApplicationMap `json:"app_map"`
}

type ApplicationMap struct {
	Application  ApplicationRef      `json:"application"`
	Instances    []InstanceRef       `json:"instances"`
	Clients      []LinkedApplication `json:"clients"`
	Dependencies []LinkedApplication `json:"dependencies"`
}

type ApplicationRef struct {
	ID       string            `json:"id"`
	Cluster  string            `json:"cluster"`
	Category string            `json:"category"`
	Custom   bool              `json:"custom"`
	Labels   map[string]string `json:"labels"`
	Status   string            `json:"status"`
	Icon     string            `json:"icon"`
}

type InstanceRef struct {
	ID     string            `json:"id"`
	Labels map[string]string `json:"labels"`
}

type LinkedApplication struct {
	ID               string            `json:"id"`
	Cluster          string            `json:"cluster"`
	Category         string            `json:"category"`
	Custom           bool              `json:"custom"`
	Status           string            `json:"status"`
	Icon             string            `json:"icon"`
	Labels           map[string]string `json:"labels"`
	LinkStatus       string            `json:"link_status"`
	LinkStatusReason string            `json:"link_status_reason"`
	LinkDirection    string            `json:"link_direction"`
	LinkStats        []string          `json:"link_stats"`
	LinkWeight       float64           `json:"link_weight"`
}

type TracingView struct {
	Spans []TracingSpan `json:"spans"`
}

type TracingSpan struct {
	Service    string            `json:"service"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

type Dashboard struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Config      DashboardConfig `json:"config"`
}

type DashboardConfig struct {
	Groups []DashboardPanelGroup `json:"groups"`
}

type DashboardPanelGroup struct {
	Name      string           `json:"name"`
	Panels    []DashboardPanel `json:"panels"`
	Collapsed bool             `json:"collapsed"`
}

type DashboardPanel struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Source      DashboardPanelSource `json:"source"`
	Widget      DashboardPanelWidget `json:"widget"`
	Box         DashboardPanelBox    `json:"box"`
}

type DashboardPanelSource struct {
	Metrics *DashboardPanelSourceMetrics `json:"metrics,omitempty"`
}

type DashboardPanelSourceMetrics struct {
	Queries []DashboardQuery `json:"queries"`
}

type DashboardQuery struct {
	DataSource string `json:"datasource"`
	Query      string `json:"query"`
	Legend     string `json:"legend"`
	Color      string `json:"color"`
}

type DashboardPanelWidget struct {
	Chart *DashboardChart `json:"chart,omitempty"`
}

type DashboardChart struct {
	Display string `json:"display"`
	Stacked bool   `json:"stacked"`
}

type DashboardPanelBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type dashboardMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type dashboardForm struct {
	Action      string          `json:"action,omitempty"`
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Config      DashboardConfig `json:"config"`
}

type envelope[T any] struct {
	Data T `json:"data"`
}

func NewClient(baseURL, email, password string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse coroot base url: %w", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &Client{
		baseURL:  u,
		email:    email,
		password: password,
		http: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
	}, nil
}

func (c *Client) Login(ctx context.Context) error {
	body, err := json.Marshal(loginRequest{
		Email:    c.email,
		Password: c.password,
	})
	if err != nil {
		return fmt.Errorf("encode login request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.resolve("/api/login"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("coroot login failed: %s", strings.TrimSpace(string(msg)))
	}
	return nil
}

func (c *Client) GetOverviewMap(ctx context.Context, project string, from, to time.Time) (OverviewView, error) {
	var out struct {
		Context struct {
			Search struct {
				Applications []OverviewApplicationRef `json:"applications"`
			} `json:"search"`
		} `json:"context"`
		Data OverviewView `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path.Join("/api/project", project, "overview", "map"), timeQuery(from, to), nil, &out); err != nil {
		return OverviewView{}, err
	}
	out.Data.SearchApplications = append([]OverviewApplicationRef(nil), out.Context.Search.Applications...)
	return out.Data, nil
}

func (c *Client) GetApplication(ctx context.Context, project, appID string, from, to time.Time) (ApplicationView, error) {
	p := path.Join("/api/project", project, "app", url.PathEscape(appID))
	var out ApplicationView
	if err := c.getData(ctx, p, timeQuery(from, to), &out); err != nil {
		return ApplicationView{}, err
	}
	return out, nil
}

func (c *Client) GetTracing(ctx context.Context, project, appID string, from, to time.Time) (TracingView, error) {
	p := path.Join("/api/project", project, "app", url.PathEscape(appID), "tracing")
	var out TracingView
	if err := c.getData(ctx, p, timeQuery(from, to), &out); err != nil {
		return TracingView{}, err
	}
	return out, nil
}

func (c *Client) ListProjects(ctx context.Context) ([]UserProject, error) {
	var out struct {
		Projects []UserProject `json:"projects"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/user", nil, nil, &out); err != nil {
		return nil, err
	}
	return append([]UserProject(nil), out.Projects...), nil
}

func (c *Client) ResolveProject(ctx context.Context, ref string) (UserProject, error) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return UserProject{}, err
	}
	for _, project := range projects {
		if project.ID == ref {
			return project, nil
		}
	}
	var match *UserProject
	for i := range projects {
		if projects[i].Name != ref {
			continue
		}
		if match != nil {
			return UserProject{}, fmt.Errorf("coroot project reference %q is ambiguous", ref)
		}
		match = &projects[i]
	}
	if match != nil {
		return *match, nil
	}
	return UserProject{}, fmt.Errorf("coroot project %q not found", ref)
}

func (c *Client) ListDashboards(ctx context.Context, project string) ([]dashboardMeta, error) {
	var out []dashboardMeta
	if err := c.getData(ctx, path.Join("/api/project", project, "dashboards"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateDashboard(ctx context.Context, project, name, description string) (string, error) {
	form := dashboardForm{
		Action:      "create",
		Name:        name,
		Description: description,
	}
	return c.postString(ctx, path.Join("/api/project", project, "dashboards"), form)
}

func (c *Client) UpdateDashboardMetadata(ctx context.Context, project, dashboardID, name, description string) error {
	form := dashboardForm{
		Action:      "update",
		Name:        name,
		Description: description,
	}
	return c.postNoContent(ctx, path.Join("/api/project", project, "dashboards", dashboardID), form)
}

func (c *Client) SaveDashboardConfig(ctx context.Context, project string, dashboard Dashboard) error {
	form := dashboardForm{
		Name:        dashboard.Name,
		Description: dashboard.Description,
		Config:      dashboard.Config,
	}
	return c.postNoContent(ctx, path.Join("/api/project", project, "dashboards", dashboard.ID), form)
}

func (c *Client) UpsertDashboard(ctx context.Context, project string, dashboard Dashboard) (string, error) {
	items, err := c.ListDashboards(ctx, project)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Name != dashboard.Name {
			continue
		}
		dashboard.ID = item.ID
		if err := c.UpdateDashboardMetadata(ctx, project, item.ID, dashboard.Name, dashboard.Description); err != nil {
			return "", err
		}
		if err := c.SaveDashboardConfig(ctx, project, dashboard); err != nil {
			return "", err
		}
		return item.ID, nil
	}
	id, err := c.CreateDashboard(ctx, project, dashboard.Name, dashboard.Description)
	if err != nil {
		return "", err
	}
	dashboard.ID = strings.TrimSpace(id)
	if err := c.SaveDashboardConfig(ctx, project, dashboard); err != nil {
		return "", err
	}
	return dashboard.ID, nil
}

func (c *Client) getData(ctx context.Context, apiPath string, query url.Values, dst any) error {
	var out envelope[json.RawMessage]
	if err := c.doJSON(ctx, http.MethodGet, apiPath, query, nil, &out); err != nil {
		return err
	}
	if len(out.Data) == 0 || string(out.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(out.Data, dst); err != nil {
		return fmt.Errorf("decode %s payload: %w", apiPath, err)
	}
	return nil
}

func (c *Client) postString(ctx context.Context, apiPath string, body any) (string, error) {
	raw, err := c.doBytes(ctx, http.MethodPost, apiPath, nil, body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) postNoContent(ctx context.Context, apiPath string, body any) error {
	_, err := c.doBytes(ctx, http.MethodPost, apiPath, nil, body)
	return err
}

func (c *Client) doJSON(ctx context.Context, method, apiPath string, query url.Values, body any, dst any) error {
	raw, err := c.doBytes(ctx, method, apiPath, query, body)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode coroot response for %s: %w", apiPath, err)
	}
	return nil
}

func (c *Client) doBytes(ctx context.Context, method, apiPath string, query url.Values, body any) ([]byte, error) {
	rawBody := []byte(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		rawBody = b
	}
	tryLogin := true
	for {
		reqURL := c.resolve(apiPath)
		if len(query) > 0 {
			parsed, err := url.Parse(reqURL)
			if err != nil {
				return nil, fmt.Errorf("parse request url: %w", err)
			}
			parsed.RawQuery = query.Encode()
			reqURL = parsed.String()
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(rawBody))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request %s %s failed: %w", method, apiPath, err)
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response body: %w", readErr)
		}
		if resp.StatusCode == http.StatusUnauthorized && tryLogin {
			tryLogin = false
			if err := c.Login(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			msg := strings.TrimSpace(string(raw))
			if msg == "" {
				msg = resp.Status
			}
			return nil, fmt.Errorf("coroot %s %s failed: %s", method, apiPath, msg)
		}
		return raw, nil
	}
}

func (c *Client) resolve(apiPath string) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + apiPath
	return u.String()
}

func timeQuery(from, to time.Time) url.Values {
	q := url.Values{}
	q.Set("from", from.UTC().Format(time.RFC3339))
	q.Set("to", to.UTC().Format(time.RFC3339))
	return q
}
