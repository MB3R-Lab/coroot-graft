package reporting

import (
	"fmt"
	"sort"
	"strings"

	"coroot-graft/internal/topology"
)

func patchEffectiveDocument(document map[string]any, report *Report) {
	if report == nil {
		return
	}
	document["runtime_activity"] = report.RuntimeActivity
	patchEndpointResults(document["endpoint_results"], report.EndpointResults)
	patchSummary(asObject(document["summary"]), report.Summary)
	patchPolicyEvaluation(asObject(document["policy_evaluation"]), report)
	patchProfiles(document["profiles"], report)
}

func patchSummary(document map[string]any, summary Summary) {
	document["overall_availability"] = summary.OverallAvailability
	document["weighted_overall_availability"] = summary.WeightedOverallAvailability
	document["cross_profile_availability"] = summary.CrossProfileAvailability
	document["cross_profile_weighted_availability"] = summary.CrossProfileWeightedAvailability
	document["risk_score"] = summary.RiskScore
	document["confidence"] = summary.Confidence
}

func patchPolicyEvaluation(document map[string]any, report *Report) {
	document["decision"] = report.PolicyEvaluation.Decision
	document["failed_endpoints"] = report.PolicyEvaluation.FailedEndpoints
	document["failed_profiles"] = report.PolicyEvaluation.FailedProfiles
	if report.RuntimeActivity != nil {
		document["reasons"] = appendRuntimeReasons(
			asArray(document["reasons"]),
			report.RuntimeActivity.ImpactedEndpoints,
			"",
			classifyRuntimeStatus(report.PolicyEvaluation.Mode),
		)
	}
	if aggregate := asObject(document["cross_profile_aggregate"]); len(aggregate) > 0 {
		patchAggregate(aggregate, report.Summary.CrossProfileWeightedAvailability, report.PolicyEvaluation.Mode)
	}
}

func patchProfiles(rawProfiles any, report *Report) {
	profilesByName := make(map[string]*Profile, len(report.Profiles))
	for i := range report.Profiles {
		profilesByName[report.Profiles[i].Name] = &report.Profiles[i]
	}
	for _, item := range asArray(rawProfiles) {
		document := asObject(item)
		profile := profilesByName[asString(document["name"])]
		if profile == nil {
			continue
		}
		simulation := asObject(document["simulation"])
		simulation["endpoint_availability"] = profile.Simulation.EndpointAvailability
		simulation["endpoint_weights"] = profile.Simulation.EndpointWeights
		simulation["weighted_aggregate"] = profile.Simulation.WeightedAggregate
		simulation["unweighted_aggregate"] = profile.Simulation.UnweightedAggregate
		patchEndpointResults(document["endpoint_results"], profile.EndpointResults)
		document["decision"] = profile.Decision
		document["endpoints_below_threshold"] = countNonPassing(profile.EndpointResults)
		if aggregate := asObject(document["aggregate"]); len(aggregate) > 0 {
			patchAggregate(aggregate, profile.Simulation.WeightedAggregate, report.PolicyEvaluation.Mode)
		}
		if report.RuntimeActivity != nil {
			document["reasons"] = appendRuntimeReasons(
				asArray(document["reasons"]),
				report.RuntimeActivity.ImpactedEndpoints,
				profile.Name,
				classifyRuntimeStatus(report.PolicyEvaluation.Mode),
			)
		}
	}
}

func patchEndpointResults(rawResults any, results []EndpointResult) {
	byKey := make(map[string]EndpointResult, len(results))
	for _, result := range results {
		byKey[result.Profile+"\x00"+result.EndpointID] = result
		byKey["\x00"+result.EndpointID] = result
	}
	for _, item := range asArray(rawResults) {
		document := asObject(item)
		key := asString(document["profile"]) + "\x00" + asString(document["endpoint_id"])
		result, ok := byKey[key]
		if !ok {
			continue
		}
		document["availability"] = result.Availability
		document["threshold"] = result.Threshold
		document["threshold_delta"] = result.Availability - result.Threshold
		document["status"] = result.Status
	}
}

func patchAggregate(document map[string]any, availability float64, mode string) {
	document["availability"] = availability
	threshold, ok := document["threshold"].(float64)
	if !ok {
		return
	}
	document["threshold_delta"] = availability - threshold
	if availability < threshold {
		document["status"] = classifyRuntimeStatus(mode)
	} else {
		document["status"] = "pass"
	}
}

func appendRuntimeReasons(reasons []any, impacted map[string][]string, profile, status string) []any {
	endpointIDs := make([]string, 0, len(impacted))
	for endpointID := range impacted {
		endpointIDs = append(endpointIDs, endpointID)
	}
	sort.Strings(endpointIDs)
	for _, endpointID := range endpointIDs {
		missing := impacted[endpointID]
		reason := map[string]any{
			"id":               "runtime_dependency_unobserved",
			"scope":            "endpoint",
			"endpoint_id":      endpointID,
			"status":           status,
			"availability":     0,
			"missing_services": missing,
			"message":          fmt.Sprintf("endpoint %q requires service(s) not observed in the runtime activity window: %s", endpointID, strings.Join(missing, ", ")),
		}
		if profile != "" {
			reason["profile"] = profile
		}
		reasons = append(reasons, reason)
	}
	return reasons
}

func countNonPassing(results []EndpointResult) int {
	count := 0
	for _, result := range results {
		if result.Status != "pass" {
			count++
		}
	}
	return count
}

func asObject(value any) map[string]any {
	document, _ := value.(map[string]any)
	if document == nil {
		document = map[string]any{}
	}
	return document
}

func asArray(value any) []any {
	items, _ := value.([]any)
	return items
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

// ApplyRuntimeImpact overlays current service observation onto a stable Sheaft
// resilience report. The topology and raw Sheaft report remain unchanged; only
// endpoints whose blocking dependency chain contains an unobserved service are
// forced to zero in the effective report exported by coroot-graft.
func ApplyRuntimeImpact(report *Report, impact topology.RuntimeImpact) {
	if report == nil {
		return
	}
	report.RuntimeActivity = &impact
	if len(impact.ImpactedEndpoints) == 0 {
		return
	}

	runtimeStatus := classifyRuntimeStatus(report.PolicyEvaluation.Mode)
	for i := range report.EndpointResults {
		if _, impacted := impact.ImpactedEndpoints[report.EndpointResults[i].EndpointID]; !impacted {
			continue
		}
		report.EndpointResults[i].Availability = 0
		report.EndpointResults[i].Status = runtimeStatus
	}

	impactedProfiles := map[string]struct{}{}
	for i := range report.Profiles {
		profile := &report.Profiles[i]
		profileImpacted := false
		for endpointID := range impact.ImpactedEndpoints {
			if _, exists := profile.Simulation.EndpointAvailability[endpointID]; exists {
				profile.Simulation.EndpointAvailability[endpointID] = 0
				profileImpacted = true
			}
		}
		for j := range profile.EndpointResults {
			if _, impacted := impact.ImpactedEndpoints[profile.EndpointResults[j].EndpointID]; !impacted {
				continue
			}
			profile.EndpointResults[j].Availability = 0
			profile.EndpointResults[j].Status = runtimeStatus
			profileImpacted = true
		}
		profile.Simulation.WeightedAggregate, profile.Simulation.UnweightedAggregate = recomputeAggregates(
			profile.Simulation.EndpointAvailability,
			profile.Simulation.EndpointWeights,
		)
		if profileImpacted {
			profile.Decision = worseDecision(profile.Decision, runtimeDecision(report.PolicyEvaluation.Mode))
			impactedProfiles[profile.Name] = struct{}{}
		}
	}

	if len(report.Profiles) > 0 {
		first := report.Profiles[0].Simulation
		report.Summary.OverallAvailability = first.UnweightedAggregate
		report.Summary.WeightedOverallAvailability = first.WeightedAggregate
		report.Summary.RiskScore = 1 - first.WeightedAggregate

		weighted := 0.0
		unweighted := 0.0
		for _, profile := range report.Profiles {
			weighted += profile.Simulation.WeightedAggregate
			unweighted += profile.Simulation.UnweightedAggregate
		}
		report.Summary.CrossProfileWeightedAvailability = weighted / float64(len(report.Profiles))
		report.Summary.CrossProfileAvailability = unweighted / float64(len(report.Profiles))
	} else {
		availability := make(map[string]float64, len(report.EndpointResults))
		for _, endpoint := range report.EndpointResults {
			availability[endpoint.EndpointID] = endpoint.Availability
		}
		weighted, unweighted := recomputeAggregates(availability, nil)
		report.Summary.OverallAvailability = unweighted
		report.Summary.WeightedOverallAvailability = weighted
		report.Summary.RiskScore = 1 - weighted
	}

	failedEndpoints := make(map[string]struct{}, len(report.PolicyEvaluation.FailedEndpoints)+len(impact.ImpactedEndpoints))
	for _, endpointID := range report.PolicyEvaluation.FailedEndpoints {
		failedEndpoints[endpointID] = struct{}{}
	}
	for endpointID := range impact.ImpactedEndpoints {
		failedEndpoints[endpointID] = struct{}{}
	}
	report.PolicyEvaluation.FailedEndpoints = sortedKeys(failedEndpoints)

	failedProfiles := make(map[string]struct{}, len(report.PolicyEvaluation.FailedProfiles)+len(impactedProfiles))
	for _, profile := range report.PolicyEvaluation.FailedProfiles {
		failedProfiles[profile] = struct{}{}
	}
	for profile := range impactedProfiles {
		failedProfiles[profile] = struct{}{}
	}
	report.PolicyEvaluation.FailedProfiles = sortedKeys(failedProfiles)
	report.PolicyEvaluation.Decision = worseDecision(
		report.PolicyEvaluation.Decision,
		runtimeDecision(report.PolicyEvaluation.Mode),
	)
}

func recomputeAggregates(availability, weights map[string]float64) (float64, float64) {
	if len(availability) == 0 {
		return 0, 0
	}
	total := 0.0
	for _, value := range availability {
		total += value
	}
	unweighted := total / float64(len(availability))

	weighted := 0.0
	totalWeight := 0.0
	for endpointID, value := range availability {
		weight := weights[endpointID]
		if weight <= 0 {
			continue
		}
		weighted += value * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return unweighted, unweighted
	}
	return weighted / totalWeight, unweighted
}

func classifyRuntimeStatus(mode string) string {
	if mode == "fail" {
		return "fail"
	}
	return "warn"
}

func runtimeDecision(mode string) string {
	switch mode {
	case "fail":
		return "fail"
	case "report":
		return "report"
	default:
		return "warn"
	}
}

func worseDecision(current, runtime string) string {
	rank := map[string]int{"pass": 0, "report": 1, "warn": 2, "fail": 3}
	if rank[current] >= rank[runtime] {
		return current
	}
	return runtime
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
