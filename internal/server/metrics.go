package server

import (
	"coroot-graft/internal/state"

	"github.com/prometheus/client_golang/prometheus"
)

type metricsCollector struct {
	store *state.Store

	lastRunSuccess       *prometheus.Desc
	lastRunTimestamp     *prometheus.Desc
	lastRunDuration      *prometheus.Desc
	gateDecision         *prometheus.Desc
	summaryOverall       *prometheus.Desc
	summaryWeighted      *prometheus.Desc
	summaryCrossProfile  *prometheus.Desc
	summaryCrossWeighted *prometheus.Desc
	summaryRisk          *prometheus.Desc
	summaryConfidence    *prometheus.Desc
	contractPolicyState  *prometheus.Desc
	profileDecision      *prometheus.Desc
	profileWeighted      *prometheus.Desc
	profileUnweighted    *prometheus.Desc
	endpointAvailability *prometheus.Desc
	endpointThreshold    *prometheus.Desc
	endpointStatus       *prometheus.Desc
}

func newMetricsCollector(store *state.Store) *metricsCollector {
	return &metricsCollector{
		store: store,
		lastRunSuccess: prometheus.NewDesc(
			"coroot_graft_last_run_success",
			"Whether the latest sync finished successfully",
			[]string{"project"}, nil,
		),
		lastRunTimestamp: prometheus.NewDesc(
			"coroot_graft_last_run_timestamp_seconds",
			"Unix timestamp of the latest completed sync",
			[]string{"project"}, nil,
		),
		lastRunDuration: prometheus.NewDesc(
			"coroot_graft_last_run_duration_seconds",
			"Duration of the latest sync",
			[]string{"project"}, nil,
		),
		gateDecision: prometheus.NewDesc(
			"coroot_graft_gate_decision_state",
			"Current Sheaft gate decision state",
			[]string{"project", "decision"}, nil,
		),
		summaryOverall: prometheus.NewDesc(
			"coroot_graft_summary_overall_availability",
			"Latest Sheaft overall availability",
			[]string{"project"}, nil,
		),
		summaryWeighted: prometheus.NewDesc(
			"coroot_graft_summary_weighted_overall_availability",
			"Latest Sheaft weighted overall availability",
			[]string{"project"}, nil,
		),
		summaryCrossProfile: prometheus.NewDesc(
			"coroot_graft_summary_cross_profile_availability",
			"Latest Sheaft cross-profile availability",
			[]string{"project"}, nil,
		),
		summaryCrossWeighted: prometheus.NewDesc(
			"coroot_graft_summary_cross_profile_weighted_availability",
			"Latest Sheaft cross-profile weighted availability",
			[]string{"project"}, nil,
		),
		summaryRisk: prometheus.NewDesc(
			"coroot_graft_summary_risk_score",
			"Latest Sheaft risk score",
			[]string{"project"}, nil,
		),
		summaryConfidence: prometheus.NewDesc(
			"coroot_graft_summary_confidence",
			"Latest Sheaft confidence",
			[]string{"project"}, nil,
		),
		contractPolicyState: prometheus.NewDesc(
			"coroot_graft_contract_policy_state",
			"Current contract policy state",
			[]string{"project", "status", "action"}, nil,
		),
		profileDecision: prometheus.NewDesc(
			"coroot_graft_profile_decision_state",
			"Per-profile decision state",
			[]string{"project", "profile", "decision"}, nil,
		),
		profileWeighted: prometheus.NewDesc(
			"coroot_graft_profile_weighted_aggregate",
			"Per-profile weighted aggregate availability",
			[]string{"project", "profile"}, nil,
		),
		profileUnweighted: prometheus.NewDesc(
			"coroot_graft_profile_unweighted_aggregate",
			"Per-profile unweighted aggregate availability",
			[]string{"project", "profile"}, nil,
		),
		endpointAvailability: prometheus.NewDesc(
			"coroot_graft_endpoint_availability",
			"Availability by endpoint and profile",
			[]string{"project", "profile", "endpoint"}, nil,
		),
		endpointThreshold: prometheus.NewDesc(
			"coroot_graft_endpoint_threshold",
			"Threshold by endpoint and profile",
			[]string{"project", "profile", "endpoint"}, nil,
		),
		endpointStatus: prometheus.NewDesc(
			"coroot_graft_endpoint_status_state",
			"Endpoint status state by endpoint and profile",
			[]string{"project", "profile", "endpoint", "status"}, nil,
		),
	}
}

func (c *metricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.lastRunSuccess
	ch <- c.lastRunTimestamp
	ch <- c.lastRunDuration
	ch <- c.gateDecision
	ch <- c.summaryOverall
	ch <- c.summaryWeighted
	ch <- c.summaryCrossProfile
	ch <- c.summaryCrossWeighted
	ch <- c.summaryRisk
	ch <- c.summaryConfidence
	ch <- c.contractPolicyState
	ch <- c.profileDecision
	ch <- c.profileWeighted
	ch <- c.profileUnweighted
	ch <- c.endpointAvailability
	ch <- c.endpointThreshold
	ch <- c.endpointStatus
}

func (c *metricsCollector) Collect(ch chan<- prometheus.Metric) {
	for _, status := range c.store.List() {
		success := 0.0
		if status.Status == "ok" {
			success = 1
		}
		ch <- prometheus.MustNewConstMetric(c.lastRunSuccess, prometheus.GaugeValue, success, status.Project)
		if !status.FinishedAt.IsZero() {
			ch <- prometheus.MustNewConstMetric(c.lastRunTimestamp, prometheus.GaugeValue, float64(status.FinishedAt.Unix()), status.Project)
		}
		if !status.StartedAt.IsZero() && !status.FinishedAt.IsZero() {
			ch <- prometheus.MustNewConstMetric(c.lastRunDuration, prometheus.GaugeValue, status.FinishedAt.Sub(status.StartedAt).Seconds(), status.Project)
		}

		report := status.Report
		if report == nil {
			continue
		}

		for _, decision := range []string{"pass", "warn", "fail", "report"} {
			value := 0.0
			if report.PolicyEvaluation.Decision == decision {
				value = 1
			}
			ch <- prometheus.MustNewConstMetric(c.gateDecision, prometheus.GaugeValue, value, status.Project, decision)
		}

		ch <- prometheus.MustNewConstMetric(c.summaryOverall, prometheus.GaugeValue, report.Summary.OverallAvailability, status.Project)
		ch <- prometheus.MustNewConstMetric(c.summaryWeighted, prometheus.GaugeValue, report.Summary.WeightedOverallAvailability, status.Project)
		ch <- prometheus.MustNewConstMetric(c.summaryCrossProfile, prometheus.GaugeValue, report.Summary.CrossProfileAvailability, status.Project)
		ch <- prometheus.MustNewConstMetric(c.summaryCrossWeighted, prometheus.GaugeValue, report.Summary.CrossProfileWeightedAvailability, status.Project)
		ch <- prometheus.MustNewConstMetric(c.summaryRisk, prometheus.GaugeValue, report.Summary.RiskScore, status.Project)
		ch <- prometheus.MustNewConstMetric(c.summaryConfidence, prometheus.GaugeValue, report.Summary.Confidence, status.Project)

		if report.ContractPolicy != nil {
			ch <- prometheus.MustNewConstMetric(
				c.contractPolicyState,
				prometheus.GaugeValue,
				1,
				status.Project,
				report.ContractPolicy.Status,
				report.ContractPolicy.Action,
			)
		}

		for _, profile := range report.Profiles {
			for _, decision := range []string{"pass", "warn", "fail", "report"} {
				value := 0.0
				if profile.Decision == decision {
					value = 1
				}
				ch <- prometheus.MustNewConstMetric(c.profileDecision, prometheus.GaugeValue, value, status.Project, profile.Name, decision)
			}
			ch <- prometheus.MustNewConstMetric(c.profileWeighted, prometheus.GaugeValue, profile.Simulation.WeightedAggregate, status.Project, profile.Name)
			ch <- prometheus.MustNewConstMetric(c.profileUnweighted, prometheus.GaugeValue, profile.Simulation.UnweightedAggregate, status.Project, profile.Name)
		}

		for _, endpoint := range report.EndpointResults {
			profile := endpoint.Profile
			if profile == "" {
				profile = "default"
			}
			ch <- prometheus.MustNewConstMetric(c.endpointAvailability, prometheus.GaugeValue, endpoint.Availability, status.Project, profile, endpoint.EndpointID)
			ch <- prometheus.MustNewConstMetric(c.endpointThreshold, prometheus.GaugeValue, endpoint.Threshold, status.Project, profile, endpoint.EndpointID)
			for _, endpointStatus := range []string{"pass", "warn", "fail"} {
				value := 0.0
				if endpoint.Status == endpointStatus {
					value = 1
				}
				ch <- prometheus.MustNewConstMetric(c.endpointStatus, prometheus.GaugeValue, value, status.Project, profile, endpoint.EndpointID, endpointStatus)
			}
		}
	}
}
