package orchestrator

import (
	"context"
	"fmt"
)

func (o *Orchestrator) InstallDashboard(ctx context.Context, projectName string) (string, error) {
	project, ok := o.projects[projectName]
	if !ok {
		return "", fmt.Errorf("unknown project %q", projectName)
	}
	resolvedProject, err := o.resolveProject(ctx, project)
	if err != nil {
		return "", err
	}
	dashboard := BuildDashboard(project)
	return o.coroot.UpsertDashboard(ctx, resolvedProject.CorootProject, dashboard)
}
