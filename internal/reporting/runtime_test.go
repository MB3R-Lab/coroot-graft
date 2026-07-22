package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"coroot-graft/internal/topology"
)

func TestApplyRuntimeImpactZeroesAffectedEndpointsAndDecision(t *testing.T) {
	report := &Report{
		EndpointResults: []EndpointResult{
			{Profile: "steady", EndpointID: "view-cart", Availability: 0.99, Threshold: 0.95, Status: "pass"},
			{Profile: "steady", EndpointID: "browse", Availability: 1, Threshold: 0.95, Status: "pass"},
		},
		Summary: Summary{
			OverallAvailability:              0.995,
			WeightedOverallAvailability:      0.995,
			CrossProfileAvailability:         0.995,
			CrossProfileWeightedAvailability: 0.995,
			RiskScore:                        0.005,
		},
		PolicyEvaluation: PolicyEvaluation{Mode: "warn", Decision: "pass"},
		Profiles: []Profile{
			{
				Name: "steady",
				Simulation: ProfileSimulation{
					EndpointAvailability: map[string]float64{"view-cart": 0.99, "browse": 1},
					EndpointWeights:      map[string]float64{"view-cart": 0.5, "browse": 0.5},
					WeightedAggregate:    0.995,
					UnweightedAggregate:  0.995,
				},
				EndpointResults: []EndpointResult{
					{Profile: "steady", EndpointID: "view-cart", Availability: 0.99, Threshold: 0.95, Status: "pass"},
					{Profile: "steady", EndpointID: "browse", Availability: 1, Threshold: 0.95, Status: "pass"},
				},
				Decision: "pass",
			},
		},
	}

	ApplyRuntimeImpact(report, topology.RuntimeImpact{
		InactiveServices:  []string{"cart"},
		ImpactedEndpoints: map[string][]string{"view-cart": {"cart"}},
	})

	if got := report.Profiles[0].Simulation.EndpointAvailability["view-cart"]; got != 0 {
		t.Fatalf("view-cart availability = %v, want 0", got)
	}
	if got := report.Profiles[0].Simulation.WeightedAggregate; got != 0.5 {
		t.Fatalf("weighted aggregate = %v, want 0.5", got)
	}
	if report.PolicyEvaluation.Decision != "warn" {
		t.Fatalf("decision = %q, want warn", report.PolicyEvaluation.Decision)
	}
	if report.EndpointResults[0].Status != "warn" {
		t.Fatalf("runtime endpoint status = %q, want warn", report.EndpointResults[0].Status)
	}
}

func TestSaveEffectivePreservesExtendedSheaftFields(t *testing.T) {
	root := t.TempDir()
	rawPath := filepath.Join(root, "sheaft-report.json")
	outputPath := filepath.Join(root, "report.json")
	raw := `{
  "endpoint_results": [{"profile":"steady","endpoint_id":"view-cart","availability":0.99,"threshold":0.95,"threshold_delta":0.04,"status":"pass","extra":"keep"}],
  "summary": {"overall_availability":0.99,"weighted_overall_availability":0.99,"cross_profile_availability":0.99,"cross_profile_weighted_availability":0.99,"risk_score":0.01,"confidence":1},
  "policy_evaluation": {"mode":"warn","decision":"pass","failed_endpoints":[],"reasons":[{"id":"existing"}]},
  "profiles": [{"name":"steady","simulation":{"endpoint_availability":{"view-cart":0.99},"endpoint_weights":{"view-cart":1},"weighted_aggregate":0.99,"unweighted_aggregate":0.99,"advanced":{"keep":true}},"endpoint_results":[{"profile":"steady","endpoint_id":"view-cart","availability":0.99,"threshold":0.95,"threshold_delta":0.04,"status":"pass"}],"decision":"pass","endpoints_below_threshold":0,"reasons":[]}],
  "provenance": {"keep":true}
}`
	if err := os.WriteFile(rawPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write raw report: %v", err)
	}
	report, err := Load(rawPath)
	if err != nil {
		t.Fatalf("load raw report: %v", err)
	}
	ApplyRuntimeImpact(report, topology.RuntimeImpact{
		InactiveServices:  []string{"cart"},
		ImpactedEndpoints: map[string][]string{"view-cart": {"cart"}},
	})
	if err := SaveEffective(rawPath, outputPath, report); err != nil {
		t.Fatalf("SaveEffective: %v", err)
	}

	var document map[string]any
	effective, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read effective report: %v", err)
	}
	if err := json.Unmarshal(effective, &document); err != nil {
		t.Fatalf("decode effective report: %v", err)
	}
	if asObject(document["provenance"])["keep"] != true {
		t.Fatalf("top-level provenance was not preserved: %+v", document)
	}
	profiles := asArray(document["profiles"])
	simulation := asObject(asObject(profiles[0])["simulation"])
	if asObject(simulation["advanced"])["keep"] != true {
		t.Fatalf("advanced simulation fields were not preserved: %+v", simulation)
	}
	endpoint := asObject(asArray(document["endpoint_results"])[0])
	if endpoint["availability"] != float64(0) || endpoint["extra"] != "keep" {
		t.Fatalf("endpoint was not patched without data loss: %+v", endpoint)
	}
	reasons := asArray(asObject(document["policy_evaluation"])["reasons"])
	if len(reasons) != 2 || asObject(reasons[1])["id"] != "runtime_dependency_unobserved" {
		t.Fatalf("runtime reason missing: %+v", reasons)
	}
}

func TestApplyRuntimeImpactOnlyDegradesProfilesContainingAffectedEndpoint(t *testing.T) {
	report := &Report{
		PolicyEvaluation: PolicyEvaluation{Mode: "warn", Decision: "pass"},
		Profiles: []Profile{
			{
				Name: "affected",
				Simulation: ProfileSimulation{
					EndpointAvailability: map[string]float64{"view-cart": 0.99},
					EndpointWeights:      map[string]float64{"view-cart": 1},
				},
				Decision: "pass",
			},
			{
				Name: "unaffected",
				Simulation: ProfileSimulation{
					EndpointAvailability: map[string]float64{"browse": 1},
					EndpointWeights:      map[string]float64{"browse": 1},
				},
				Decision: "pass",
			},
		},
	}

	ApplyRuntimeImpact(report, topology.RuntimeImpact{
		ImpactedEndpoints: map[string][]string{"view-cart": {"cart"}},
	})

	if report.Profiles[0].Decision != "warn" {
		t.Fatalf("affected profile decision = %q, want warn", report.Profiles[0].Decision)
	}
	if report.Profiles[1].Decision != "pass" {
		t.Fatalf("unaffected profile decision = %q, want pass", report.Profiles[1].Decision)
	}
	if len(report.PolicyEvaluation.FailedProfiles) != 1 || report.PolicyEvaluation.FailedProfiles[0] != "affected" {
		t.Fatalf("failed profiles = %v, want only affected", report.PolicyEvaluation.FailedProfiles)
	}
}

func TestWorseDecisionDoesNotDowngradeFailureInReportMode(t *testing.T) {
	if got := worseDecision("fail", "report"); got != "fail" {
		t.Fatalf("worseDecision(fail, report) = %q, want fail", got)
	}
	if got := worseDecision("pass", "report"); got != "report" {
		t.Fatalf("worseDecision(pass, report) = %q, want report", got)
	}
}
