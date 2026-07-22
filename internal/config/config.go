package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultListenAddress  = ":8095"
	DefaultStorageDir     = ".coroot-graft"
	DefaultSyncTimeout    = 10 * time.Minute
	DefaultHTTPTimeout    = 30 * time.Second
	DefaultTimeWindow     = time.Hour
	DefaultActivityWindow = 2 * time.Minute
)

type Config struct {
	ListenAddress string          `yaml:"listen_address"`
	StorageDir    string          `yaml:"storage_dir"`
	SyncTimeout   time.Duration   `yaml:"sync_timeout"`
	Coroot        CorootConfig    `yaml:"coroot"`
	Toolchain     ToolchainConfig `yaml:"toolchain"`
	Projects      []ProjectConfig `yaml:"projects"`
}

type CorootConfig struct {
	BaseURL        string        `yaml:"base_url"`
	Email          string        `yaml:"email"`
	Password       string        `yaml:"password"`
	HTTPTimeout    time.Duration `yaml:"http_timeout"`
	TimeWindow     time.Duration `yaml:"time_window"`
	ActivityWindow time.Duration `yaml:"activity_window"`
}

type ToolchainConfig struct {
	Bering CommandConfig `yaml:"bering"`
	Sheaft CommandConfig `yaml:"sheaft"`
}

type CommandConfig struct {
	Command    []string          `yaml:"command"`
	WorkingDir string            `yaml:"working_dir"`
	Env        map[string]string `yaml:"env"`
}

type ProjectConfig struct {
	Name               string          `yaml:"name"`
	CorootProject      string          `yaml:"coroot_project"`
	Interval           time.Duration   `yaml:"interval"`
	TimeWindow         time.Duration   `yaml:"time_window"`
	ActivityWindow     time.Duration   `yaml:"activity_window"`
	OverlayPath        string          `yaml:"overlay"`
	AnalysisPath       string          `yaml:"analysis"`
	PolicyPath         string          `yaml:"policy"`
	ContractPolicyPath string          `yaml:"contract_policy"`
	JourneysPath       string          `yaml:"journeys"`
	EndpointMode       string          `yaml:"endpoint_mode"`
	IncludeExternal    bool            `yaml:"include_external"`
	IncludeApps        []string        `yaml:"include_apps"`
	ExcludeCategories  []string        `yaml:"exclude_categories"`
	EdgeOverrides      []EdgeOverride  `yaml:"edge_overrides"`
	WebhookSecret      string          `yaml:"webhook_secret"`
	Dashboard          DashboardConfig `yaml:"dashboard"`
}

type EdgeOverride struct {
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	Kind     string `yaml:"kind"`
	Blocking *bool  `yaml:"blocking"`
}

type DashboardConfig struct {
	Install     bool   `yaml:"install"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	raw = []byte(os.ExpandEnv(string(raw)))

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config yaml: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.normalize(filepath.Dir(path)); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.ListenAddress == "" {
		c.ListenAddress = DefaultListenAddress
	}
	if c.StorageDir == "" {
		c.StorageDir = DefaultStorageDir
	}
	if c.SyncTimeout <= 0 {
		c.SyncTimeout = DefaultSyncTimeout
	}
	if c.Coroot.HTTPTimeout <= 0 {
		c.Coroot.HTTPTimeout = DefaultHTTPTimeout
	}
	if c.Coroot.TimeWindow <= 0 {
		c.Coroot.TimeWindow = DefaultTimeWindow
	}
	if c.Coroot.ActivityWindow <= 0 {
		c.Coroot.ActivityWindow = DefaultActivityWindow
	}
	if len(c.Toolchain.Bering.Command) == 0 {
		c.Toolchain.Bering.Command = []string{"bering"}
	}
	if len(c.Toolchain.Sheaft.Command) == 0 {
		c.Toolchain.Sheaft.Command = []string{"sheaft"}
	}
	if c.Toolchain.Bering.Env == nil {
		c.Toolchain.Bering.Env = map[string]string{}
	}
	if c.Toolchain.Sheaft.Env == nil {
		c.Toolchain.Sheaft.Env = map[string]string{}
	}

	for i := range c.Projects {
		p := &c.Projects[i]
		if p.CorootProject == "" {
			p.CorootProject = p.Name
		}
		if p.EndpointMode == "" {
			p.EndpointMode = "service"
		}
		if len(p.ExcludeCategories) == 0 {
			p.ExcludeCategories = []string{"monitoring"}
		}
		if p.Dashboard.Name == "" {
			p.Dashboard.Name = "Coroot Graft"
		}
		if p.Dashboard.Description == "" {
			p.Dashboard.Description = "Managed resilience dashboard published by coroot-graft"
		}
	}
}

func (c *Config) normalize(baseDir string) error {
	var err error
	c.StorageDir, err = resolvePath(baseDir, c.StorageDir)
	if err != nil {
		return fmt.Errorf("storage_dir: %w", err)
	}
	c.Toolchain.Bering.WorkingDir, err = resolvePath(baseDir, c.Toolchain.Bering.WorkingDir)
	if err != nil {
		return fmt.Errorf("toolchain.bering.working_dir: %w", err)
	}
	c.Toolchain.Sheaft.WorkingDir, err = resolvePath(baseDir, c.Toolchain.Sheaft.WorkingDir)
	if err != nil {
		return fmt.Errorf("toolchain.sheaft.working_dir: %w", err)
	}
	for i := range c.Projects {
		p := &c.Projects[i]
		if p.OverlayPath, err = resolvePath(baseDir, p.OverlayPath); err != nil {
			return fmt.Errorf("projects[%d].overlay: %w", i, err)
		}
		if p.AnalysisPath, err = resolvePath(baseDir, p.AnalysisPath); err != nil {
			return fmt.Errorf("projects[%d].analysis: %w", i, err)
		}
		if p.PolicyPath, err = resolvePath(baseDir, p.PolicyPath); err != nil {
			return fmt.Errorf("projects[%d].policy: %w", i, err)
		}
		if p.ContractPolicyPath, err = resolvePath(baseDir, p.ContractPolicyPath); err != nil {
			return fmt.Errorf("projects[%d].contract_policy: %w", i, err)
		}
		if p.JourneysPath, err = resolvePath(baseDir, p.JourneysPath); err != nil {
			return fmt.Errorf("projects[%d].journeys: %w", i, err)
		}
	}
	return nil
}

func (c Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen_address cannot be empty")
	}
	if c.StorageDir == "" {
		return errors.New("storage_dir cannot be empty")
	}
	if c.SyncTimeout <= 0 {
		return errors.New("sync_timeout must be > 0")
	}
	if err := c.Coroot.Validate(); err != nil {
		return err
	}
	if err := c.Toolchain.Validate(); err != nil {
		return err
	}
	if len(c.Projects) == 0 {
		return errors.New("at least one project is required")
	}
	seen := map[string]struct{}{}
	for i, p := range c.Projects {
		if _, ok := seen[p.Name]; ok {
			return fmt.Errorf("duplicate project name: %s", p.Name)
		}
		seen[p.Name] = struct{}{}
		if err := p.Validate(i); err != nil {
			return err
		}
		topologyWindow := c.Coroot.TimeWindow
		if p.TimeWindow > 0 {
			topologyWindow = p.TimeWindow
		}
		activityWindow := c.Coroot.ActivityWindow
		if p.ActivityWindow > 0 {
			activityWindow = p.ActivityWindow
		}
		if activityWindow > topologyWindow {
			return fmt.Errorf("projects[%d].activity_window (%s) must not exceed time_window (%s)", i, activityWindow, topologyWindow)
		}
	}
	return nil
}

func (c CorootConfig) Validate() error {
	if c.BaseURL == "" {
		return errors.New("coroot.base_url cannot be empty")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("coroot.base_url is invalid: %q", c.BaseURL)
	}
	if c.Email == "" {
		return errors.New("coroot.email cannot be empty")
	}
	if c.Password == "" {
		return errors.New("coroot.password cannot be empty")
	}
	if c.HTTPTimeout <= 0 {
		return errors.New("coroot.http_timeout must be > 0")
	}
	if c.TimeWindow <= 0 {
		return errors.New("coroot.time_window must be > 0")
	}
	if c.ActivityWindow <= 0 {
		return errors.New("coroot.activity_window must be > 0")
	}
	return nil
}

func (c ToolchainConfig) Validate() error {
	if err := c.Bering.Validate("toolchain.bering"); err != nil {
		return err
	}
	if err := c.Sheaft.Validate("toolchain.sheaft"); err != nil {
		return err
	}
	return nil
}

func (c CommandConfig) Validate(scope string) error {
	if len(c.Command) == 0 {
		return fmt.Errorf("%s.command must contain at least one element", scope)
	}
	for i, part := range c.Command {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("%s.command[%d] cannot be empty", scope, i)
		}
	}
	return nil
}

func (p ProjectConfig) Validate(idx int) error {
	scope := fmt.Sprintf("projects[%d]", idx)
	if p.Name == "" {
		return fmt.Errorf("%s.name cannot be empty", scope)
	}
	if p.CorootProject == "" {
		return fmt.Errorf("%s.coroot_project cannot be empty", scope)
	}
	if p.Interval < 0 {
		return fmt.Errorf("%s.interval must be >= 0", scope)
	}
	if p.TimeWindow < 0 {
		return fmt.Errorf("%s.time_window must be >= 0", scope)
	}
	if p.ActivityWindow < 0 {
		return fmt.Errorf("%s.activity_window must be >= 0", scope)
	}
	if p.AnalysisPath == "" && p.PolicyPath == "" {
		return fmt.Errorf("%s must define analysis or policy", scope)
	}
	if p.AnalysisPath != "" && p.PolicyPath != "" {
		return fmt.Errorf("%s cannot define both analysis and policy", scope)
	}
	switch p.EndpointMode {
	case "service", "trace_http":
	default:
		return fmt.Errorf("%s.endpoint_mode must be one of service, trace_http", scope)
	}
	for j, item := range p.IncludeApps {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("%s.include_apps[%d] cannot be empty", scope, j)
		}
	}
	for j, ov := range p.EdgeOverrides {
		if ov.From == "" || ov.To == "" {
			return fmt.Errorf("%s.edge_overrides[%d] requires from and to", scope, j)
		}
		if ov.Kind != "" && ov.Kind != "sync" && ov.Kind != "async" {
			return fmt.Errorf("%s.edge_overrides[%d].kind must be sync or async", scope, j)
		}
	}
	if p.PolicyPath == "" && p.JourneysPath != "" {
		return fmt.Errorf("%s.journeys is only supported with policy mode; embed journeys in analysis config instead", scope)
	}
	return nil
}

func resolvePath(baseDir, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	if baseDir == "" {
		return filepath.Clean(value), nil
	}
	return filepath.Abs(filepath.Join(baseDir, value))
}
