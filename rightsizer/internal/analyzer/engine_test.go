package analyzer

import (
	"testing"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

func defaultConfig() models.Config {
	return models.Config{
		HeadroomFactor:  0.20,
		SpikePercentile: "P95",
		CostPerCPUHour:  0.032,
		CostPerGiBHour:  0.004,
		MockMode:        true,
	}
}

func makeRawWorkload(cpuReq, memReq, cpuLim, memLim, usageFraction float64) models.RawWorkload {
	baseCPU := cpuReq * usageFraction
	baseMem := memReq * usageFraction
	return models.RawWorkload{
		Name:      "test-app",
		Namespace: "default",
		Type:      models.Deployment,
		Replicas:  2,
		Containers: []models.RawContainer{
			{
				Name:           "main",
				Image:          "test:latest",
				CurrentRequest: models.ResourceValues{CPUCores: cpuReq, MemoryGiB: memReq},
				CurrentLimit:   models.ResourceValues{CPUCores: cpuLim, MemoryGiB: memLim},
				Usage: models.UsageStats{
					Average: models.ResourceValues{CPUCores: baseCPU, MemoryGiB: baseMem},
					P50:     models.ResourceValues{CPUCores: baseCPU * 0.95, MemoryGiB: baseMem * 0.95},
					P90:     models.ResourceValues{CPUCores: baseCPU * 1.20, MemoryGiB: baseMem * 1.15},
					P95:     models.ResourceValues{CPUCores: baseCPU * 1.35, MemoryGiB: baseMem * 1.25},
					P99:     models.ResourceValues{CPUCores: baseCPU * 1.55, MemoryGiB: baseMem * 1.40},
					Max:     models.ResourceValues{CPUCores: baseCPU * 1.80, MemoryGiB: baseMem * 1.60},
				},
			},
		},
	}
}

func TestAnalyzeWorkload(t *testing.T) {
	engine := NewEngine(defaultConfig())
	raw := makeRawWorkload(1.0, 2.0, 2.0, 4.0, 0.30)

	result := engine.AnalyzeWorkload(raw)

	if result.ID != "default/test-app" {
		t.Errorf("expected ID 'default/test-app', got %q", result.ID)
	}
	if result.Name != "test-app" {
		t.Errorf("expected Name 'test-app', got %q", result.Name)
	}
	if result.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %q", result.Namespace)
	}
	if result.Type != models.Deployment {
		t.Errorf("expected Type Deployment, got %q", result.Type)
	}
	if result.Replicas != 2 {
		t.Errorf("expected Replicas 2, got %d", result.Replicas)
	}
	if len(result.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(result.Containers))
	}

	c := result.Containers[0]
	if c.Name != "main" {
		t.Errorf("expected container name 'main', got %q", c.Name)
	}
	if c.Recommended.Request.CPUCores <= 0 {
		t.Error("recommended CPU request should be > 0")
	}
	if c.Recommended.Request.MemoryGiB <= 0 {
		t.Error("recommended memory request should be > 0")
	}
	if c.Recommended.Limit.CPUCores <= c.Recommended.Request.CPUCores {
		t.Error("recommended CPU limit should exceed recommended CPU request")
	}
	if c.Recommended.YAMLPatch == "" {
		t.Error("expected non-empty YAMLPatch")
	}
	if c.Recommended.KubectlCmd == "" {
		t.Error("expected non-empty KubectlCmd")
	}
	if c.ConfidenceScore <= 0 {
		t.Error("expected positive confidence score")
	}
	if result.LastAnalyzed.IsZero() {
		t.Error("expected LastAnalyzed to be set")
	}
	if time.Since(result.LastAnalyzed) > 5*time.Second {
		t.Error("LastAnalyzed should be recent")
	}
}

func TestComputeClusterSummary(t *testing.T) {
	engine := NewEngine(defaultConfig())

	raws := []models.RawWorkload{
		makeRawWorkload(1.0, 2.0, 2.0, 4.0, 0.30),
		makeRawWorkload(2.0, 4.0, 4.0, 8.0, 0.50),
	}
	raws[1].Namespace = "other"
	raws[1].Name = "other-app"

	var analyzed []models.WorkloadAnalysis
	for _, r := range raws {
		analyzed = append(analyzed, engine.AnalyzeWorkload(r))
	}

	summary := engine.ComputeClusterSummary(analyzed)

	if summary.TotalWorkloads != 2 {
		t.Errorf("expected 2 workloads, got %d", summary.TotalWorkloads)
	}
	if summary.TotalContainers != 2 {
		t.Errorf("expected 2 containers, got %d", summary.TotalContainers)
	}
	if summary.CPURequestedCores <= 0 {
		t.Error("expected positive CPURequestedCores")
	}
	if summary.MemRequestedGiB <= 0 {
		t.Error("expected positive MemRequestedGiB")
	}
	if summary.CPUUsedCores <= 0 {
		t.Error("expected positive CPUUsedCores")
	}
	if summary.MemUsedGiB <= 0 {
		t.Error("expected positive MemUsedGiB")
	}
	if summary.RiskDistribution == nil {
		t.Fatal("expected non-nil RiskDistribution")
	}
	if len(summary.NamespaceSummaries) != 2 {
		t.Errorf("expected 2 namespace summaries, got %d", len(summary.NamespaceSummaries))
	}
}

func TestWasteScoreCalculation(t *testing.T) {
	engine := NewEngine(defaultConfig())

	tests := []struct {
		name         string
		usageFactor  float64
		expectHigher float64
		expectLower  float64
	}{
		{name: "over_provisioned", usageFactor: 0.10, expectHigher: 40, expectLower: 100},
		{name: "right_sized", usageFactor: 0.80, expectHigher: 0, expectLower: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := makeRawWorkload(1.0, 2.0, 2.0, 4.0, tt.usageFactor)
			result := engine.AnalyzeWorkload(raw)

			waste := result.OverallWaste
			if waste < tt.expectHigher || waste > tt.expectLower {
				t.Errorf("waste score %.2f not in expected range [%.0f, %.0f]", waste, tt.expectHigher, tt.expectLower)
			}
		})
	}

	// Under-provisioned: usage > request → waste should be 0
	t.Run("under_provisioned", func(t *testing.T) {
		raw := makeRawWorkload(0.5, 1.0, 1.0, 2.0, 1.20)
		result := engine.AnalyzeWorkload(raw)
		if result.OverallWaste > 10 {
			t.Errorf("under-provisioned workload should have low waste, got %.2f", result.OverallWaste)
		}
	})
}

func TestRiskLevelAssignment(t *testing.T) {
	engine := NewEngine(defaultConfig())

	tests := []struct {
		name        string
		usageFactor float64
		expectRisk  models.RiskLevel
	}{
		// Very over-provisioned: large reduction → high/critical risk
		{name: "heavily_over_provisioned", usageFactor: 0.05, expectRisk: models.High},
		// Slightly over-provisioned: small reduction → low risk
		{name: "slightly_over_provisioned", usageFactor: 0.70, expectRisk: models.Low},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := makeRawWorkload(2.0, 4.0, 4.0, 8.0, tt.usageFactor)
			result := engine.AnalyzeWorkload(raw)

			riskOrder := map[models.RiskLevel]int{
				models.Low: 0, models.Medium: 1, models.High: 2, models.Critical: 3,
			}

			if tt.name == "heavily_over_provisioned" {
				if riskOrder[result.OverallRisk] < riskOrder[models.High] {
					t.Errorf("expected risk >= HIGH for heavily over-provisioned, got %s", result.OverallRisk)
				}
			}
			if tt.name == "slightly_over_provisioned" {
				if riskOrder[result.OverallRisk] > riskOrder[models.Low] {
					t.Errorf("expected risk <= LOW for slightly over-provisioned, got %s", result.OverallRisk)
				}
			}
		})
	}
}

func TestIssueDetection(t *testing.T) {
	engine := NewEngine(defaultConfig())

	t.Run("no_limits", func(t *testing.T) {
		raw := makeRawWorkload(1.0, 2.0, 0.0, 0.0, 0.30)
		result := engine.AnalyzeWorkload(raw)
		if !containsIssue(result.Containers[0].Issues, "no_limits") {
			t.Errorf("expected 'no_limits' issue, got %v", result.Containers[0].Issues)
		}
	})

	t.Run("under_provisioned", func(t *testing.T) {
		// P95 usage will exceed request when usageFactor > ~0.74 (1/1.35)
		raw := makeRawWorkload(0.5, 1.0, 1.0, 2.0, 1.20)
		result := engine.AnalyzeWorkload(raw)
		if !containsIssue(result.Containers[0].Issues, "under_provisioned") {
			t.Errorf("expected 'under_provisioned' issue, got %v", result.Containers[0].Issues)
		}
	})

	t.Run("over_provisioned", func(t *testing.T) {
		raw := makeRawWorkload(4.0, 8.0, 8.0, 16.0, 0.10)
		result := engine.AnalyzeWorkload(raw)
		if !containsIssue(result.Containers[0].Issues, "over_provisioned") {
			t.Errorf("expected 'over_provisioned' issue, got %v", result.Containers[0].Issues)
		}
	})
}

func containsIssue(issues []string, target string) bool {
	for _, issue := range issues {
		if issue == target {
			return true
		}
	}
	return false
}
