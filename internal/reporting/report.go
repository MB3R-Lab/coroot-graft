package reporting

import (
	"encoding/json"
	"fmt"
	"os"

	"coroot-graft/internal/topology"
)

type Report struct {
	Simulation       Simulation              `json:"simulation"`
	EndpointResults  []EndpointResult        `json:"endpoint_results"`
	Summary          Summary                 `json:"summary"`
	PolicyEvaluation PolicyEvaluation        `json:"policy_evaluation"`
	ContractPolicy   *ContractPolicy         `json:"contract_policy,omitempty"`
	Profiles         []Profile               `json:"profiles,omitempty"`
	GeneratedAt      string                  `json:"generated_at,omitempty"`
	RuntimeActivity  *topology.RuntimeImpact `json:"runtime_activity,omitempty"`
}

type Simulation struct {
	Trials             int     `json:"trials"`
	Seed               int64   `json:"seed"`
	FailureProbability float64 `json:"failure_probability"`
}

type EndpointResult struct {
	Profile      string  `json:"profile,omitempty"`
	EndpointID   string  `json:"endpoint_id"`
	Availability float64 `json:"availability"`
	Threshold    float64 `json:"threshold"`
	Status       string  `json:"status"`
}

type Summary struct {
	OverallAvailability              float64 `json:"overall_availability"`
	WeightedOverallAvailability      float64 `json:"weighted_overall_availability,omitempty"`
	CrossProfileAvailability         float64 `json:"cross_profile_availability,omitempty"`
	CrossProfileWeightedAvailability float64 `json:"cross_profile_weighted_availability,omitempty"`
	RiskScore                        float64 `json:"risk_score"`
	Confidence                       float64 `json:"confidence"`
}

type PolicyEvaluation struct {
	Mode            string   `json:"mode"`
	Decision        string   `json:"decision"`
	FailedEndpoints []string `json:"failed_endpoints"`
	FailedProfiles  []string `json:"failed_profiles,omitempty"`
	EvaluationRule  string   `json:"evaluation_rule,omitempty"`
}

type ContractPolicy struct {
	Status  string `json:"status"`
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
}

type Profile struct {
	Name            string            `json:"name"`
	Simulation      ProfileSimulation `json:"simulation"`
	EndpointResults []EndpointResult  `json:"endpoint_results"`
	Decision        string            `json:"decision"`
}

type ProfileSimulation struct {
	Name                 string             `json:"name"`
	Trials               int                `json:"trials"`
	Seed                 int64              `json:"seed"`
	SamplingMode         string             `json:"sampling_mode"`
	FailureProbability   float64            `json:"failure_probability,omitempty"`
	FixedKFailures       int                `json:"fixed_k_failures,omitempty"`
	EndpointAvailability map[string]float64 `json:"endpoint_availability"`
	EndpointWeights      map[string]float64 `json:"endpoint_weights,omitempty"`
	WeightedAggregate    float64            `json:"weighted_aggregate"`
	UnweightedAggregate  float64            `json:"unweighted_aggregate"`
}

func Load(path string) (*Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var rep Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, fmt.Errorf("decode report json: %w", err)
	}
	return &rep, nil
}

func Save(path string, report *Report) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report json: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write report json: %w", err)
	}
	return nil
}

// SaveEffective preserves the full upstream Sheaft report surface while
// patching only the fields changed by the runtime activity overlay.
func SaveEffective(rawPath, outputPath string, report *Report) error {
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		return fmt.Errorf("read raw report json: %w", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("decode raw report json: %w", err)
	}
	patchEffectiveDocument(document, report)
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode effective report json: %w", err)
	}
	if err := os.WriteFile(outputPath, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write effective report json: %w", err)
	}
	return nil
}
