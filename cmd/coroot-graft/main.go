package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"coroot-graft/internal/config"
	"coroot-graft/internal/orchestrator"
	"coroot-graft/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "install-dashboard":
		runInstallDashboard(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "configs/graft.example.yaml", "path to config file")
	_ = fs.Parse(args)

	cfg, orch := mustBootstrap(*configPath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg.ListenAddress, orch)
	if err := srv.Serve(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	configPath := fs.String("config", "configs/graft.example.yaml", "path to config file")
	project := fs.String("project", "", "project name; if omitted, sync all configured projects")
	_ = fs.Parse(args)

	_, orch := mustBootstrap(*configPath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *project == "" {
		if err := orch.SyncAll(ctx, "cli"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, status := range orch.Store().List() {
			fmt.Printf("%s: %s\n", status.Project, status.Status)
		}
		return
	}

	status, err := orch.SyncProject(ctx, *project, "cli")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %s\n", status.Project, status.Status)
}

func runInstallDashboard(args []string) {
	fs := flag.NewFlagSet("install-dashboard", flag.ExitOnError)
	configPath := fs.String("config", "configs/graft.example.yaml", "path to config file")
	project := fs.String("project", "", "project name; if omitted, install for all configured projects")
	_ = fs.Parse(args)

	cfg, orch := mustBootstrap(*configPath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	projects := cfg.Projects
	if *project != "" {
		p, ok := orch.Project(*project)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown project: %s\n", *project)
			os.Exit(1)
		}
		projects = []config.ProjectConfig{p}
	}

	for _, item := range projects {
		id, err := orch.InstallDashboard(ctx, item.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s: dashboard %s\n", item.Name, id)
	}
}

func mustBootstrap(configPath string) (config.Config, *orchestrator.Orchestrator) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	orch, err := orchestrator.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return cfg, orch
}

func usage() {
	fmt.Print(`coroot-graft

Usage:
  coroot-graft serve -config <path>
  coroot-graft sync -config <path> [-project <name>]
  coroot-graft install-dashboard -config <path> [-project <name>]
`)
}
