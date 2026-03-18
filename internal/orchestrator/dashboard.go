package orchestrator

import (
	"fmt"

	"coroot-graft/internal/config"
	"coroot-graft/internal/coroot"
)

func BuildDashboard(project config.ProjectConfig) coroot.Dashboard {
	projectFilter := fmt.Sprintf(`{project=%q}`, project.Name)
	return coroot.Dashboard{
		Name:        project.Dashboard.Name,
		Description: project.Dashboard.Description,
		Config: coroot.DashboardConfig{
			Groups: []coroot.DashboardPanelGroup{
				{
					Name: "Gate",
					Panels: []coroot.DashboardPanel{
						linePanel("Gate decision state", "Current pass/warn/fail state exported by coroot-graft", 0, 0, 6, 3,
							query(`coroot_graft_gate_decision_state%s`, projectFilter, "{{ decision }}", "#2563eb"),
						),
						linePanel("Last run status", "1 when the latest sync finished successfully", 6, 0, 6, 3,
							query(`coroot_graft_last_run_success%s`, projectFilter, "success", "#16a34a"),
							query(`coroot_graft_last_run_timestamp_seconds%s`, projectFilter, "timestamp", "#ea580c"),
						),
					},
				},
				{
					Name: "Summary",
					Panels: []coroot.DashboardPanel{
						linePanel("Risk score", "Latest Sheaft risk score", 0, 0, 6, 3,
							query(`coroot_graft_summary_risk_score%s`, projectFilter, "risk", "#dc2626"),
						),
						linePanel("Cross-profile availability", "Cross-profile weighted and unweighted availability", 6, 0, 6, 3,
							query(`coroot_graft_summary_cross_profile_weighted_availability%s`, projectFilter, "weighted", "#0284c7"),
							query(`coroot_graft_summary_cross_profile_availability%s`, projectFilter, "unweighted", "#7c3aed"),
						),
						linePanel("Confidence", "Bering model confidence propagated into Sheaft report", 0, 3, 6, 3,
							query(`coroot_graft_summary_confidence%s`, projectFilter, "confidence", "#0f766e"),
						),
						linePanel("Contract policy", "Current/deprecated contract policy state", 6, 3, 6, 3,
							query(`coroot_graft_contract_policy_state%s`, projectFilter, "{{ status }}/{{ action }}", "#ca8a04"),
						),
					},
				},
				{
					Name: "Profiles",
					Panels: []coroot.DashboardPanel{
						linePanel("Profile decisions", "Per-profile pass/warn/fail state", 0, 0, 6, 3,
							query(`coroot_graft_profile_decision_state%s`, projectFilter, "{{ profile }} {{ decision }}", "#1d4ed8"),
						),
						linePanel("Profile aggregates", "Per-profile weighted aggregate availability", 6, 0, 6, 3,
							query(`coroot_graft_profile_weighted_aggregate%s`, projectFilter, "{{ profile }} weighted", "#0f766e"),
							query(`coroot_graft_profile_unweighted_aggregate%s`, projectFilter, "{{ profile }} unweighted", "#b45309"),
						),
					},
				},
				{
					Name: "Endpoints",
					Panels: []coroot.DashboardPanel{
						linePanel("Endpoint availability", "Availability by endpoint and profile", 0, 0, 12, 4,
							query(`coroot_graft_endpoint_availability%s`, projectFilter, "{{ profile }} {{ endpoint }}", "#2563eb"),
						),
						linePanel("Endpoint threshold", "Threshold by endpoint and profile", 0, 4, 12, 4,
							query(`coroot_graft_endpoint_threshold%s`, projectFilter, "{{ profile }} {{ endpoint }}", "#dc2626"),
						),
					},
				},
			},
		},
	}
}

func linePanel(name, description string, x, y, w, h int, queries ...coroot.DashboardQuery) coroot.DashboardPanel {
	return coroot.DashboardPanel{
		Name:        name,
		Description: description,
		Source: coroot.DashboardPanelSource{
			Metrics: &coroot.DashboardPanelSourceMetrics{Queries: queries},
		},
		Widget: coroot.DashboardPanelWidget{
			Chart: &coroot.DashboardChart{
				Display: "line",
			},
		},
		Box: coroot.DashboardPanelBox{X: x, Y: y, W: w, H: h},
	}
}

func query(format, filter, legend, color string) coroot.DashboardQuery {
	return coroot.DashboardQuery{
		Query:  fmt.Sprintf(format, filter),
		Legend: legend,
		Color:  color,
	}
}
