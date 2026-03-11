package mock

import (
	"testing"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

func testConfig() models.Config {
	return models.Config{
		HeadroomFactor:  0.20,
		SpikePercentile: "P95",
		CostPerCPUHour:  0.032,
		CostPerGiBHour:  0.004,
		MockMode:        true,
	}
}

func TestNewMockDataProvider(t *testing.T) {
	m := NewMockDataProvider(testConfig())
	if m == nil {
		t.Fatal("expected non-nil MockDataProvider")
	}
	if m.rng == nil {
		t.Error("expected rng to be initialized")
	}
	if len(m.namespaces) == 0 {
		t.Error("expected namespaces to be populated")
	}
	if len(m.workloads) == 0 {
		t.Error("expected workloads to be generated")
	}
}

func TestGetNamespaces(t *testing.T) {
	m := NewMockDataProvider(testConfig())
	ns := m.GetNamespaces()

	if len(ns) != 7 {
		t.Errorf("expected 7 namespaces, got %d", len(ns))
	}

	expected := map[string]bool{
		"production":    false,
		"staging":       false,
		"monitoring":    false,
		"kube-system":   false,
		"data-pipeline": false,
		"frontend":      false,
		"backend":       false,
	}

	for _, n := range ns {
		if _, ok := expected[n]; !ok {
			t.Errorf("unexpected namespace: %s", n)
		}
		expected[n] = true
	}

	for n, found := range expected {
		if !found {
			t.Errorf("missing expected namespace: %s", n)
		}
	}

	// Verify returned slice is a copy (modifying it shouldn't affect provider)
	ns[0] = "modified"
	ns2 := m.GetNamespaces()
	if ns2[0] == "modified" {
		t.Error("GetNamespaces should return a copy, not a reference")
	}
}

func TestGetWorkloads(t *testing.T) {
	m := NewMockDataProvider(testConfig())

	prodWorkloads := m.GetWorkloads("production")
	if len(prodWorkloads) == 0 {
		t.Error("expected workloads in production namespace")
	}
	for _, w := range prodWorkloads {
		if w.Namespace != "production" {
			t.Errorf("expected namespace 'production', got %q", w.Namespace)
		}
		if w.Name == "" {
			t.Error("expected non-empty workload name")
		}
		if len(w.Containers) == 0 {
			t.Errorf("workload %q has no containers", w.Name)
		}
		if w.Replicas <= 0 {
			t.Errorf("workload %q has invalid replicas: %d", w.Name, w.Replicas)
		}
	}

	// Nonexistent namespace should return nil/empty
	empty := m.GetWorkloads("nonexistent")
	if len(empty) != 0 {
		t.Errorf("expected 0 workloads for nonexistent namespace, got %d", len(empty))
	}

	// Each namespace should have at least one workload
	for _, ns := range m.GetNamespaces() {
		wl := m.GetWorkloads(ns)
		if len(wl) == 0 {
			t.Errorf("expected workloads in namespace %q", ns)
		}
	}
}

func TestGetAllWorkloads(t *testing.T) {
	m := NewMockDataProvider(testConfig())
	all := m.GetAllWorkloads()

	if len(all) < 30 || len(all) > 50 {
		t.Errorf("expected 30-50 workloads, got %d", len(all))
	}

	// Verify every workload has valid fields
	for _, w := range all {
		if w.Name == "" {
			t.Error("workload has empty name")
		}
		if w.Namespace == "" {
			t.Error("workload has empty namespace")
		}
		if w.Type != models.Deployment && w.Type != models.StatefulSet {
			t.Errorf("unexpected workload type: %q", w.Type)
		}
		if len(w.Containers) == 0 {
			t.Errorf("workload %q has no containers", w.Name)
		}
		for _, c := range w.Containers {
			if c.Name == "" {
				t.Errorf("container in workload %q has empty name", w.Name)
			}
			if c.Image == "" {
				t.Errorf("container %q in workload %q has empty image", c.Name, w.Name)
			}
		}
	}

	// Verify the sum of per-namespace workloads equals total
	total := 0
	for _, ns := range m.GetNamespaces() {
		total += len(m.GetWorkloads(ns))
	}
	if total != len(all) {
		t.Errorf("sum of per-namespace workloads (%d) != total (%d)", total, len(all))
	}

	// Verify returned slice is a copy
	all[0].Name = "modified"
	all2 := m.GetAllWorkloads()
	if all2[0].Name == "modified" {
		t.Error("GetAllWorkloads should return a copy, not a reference")
	}
}

func TestGetWorkloadMetrics(t *testing.T) {
	m := NewMockDataProvider(testConfig())

	// Test with known workload
	metrics := m.GetWorkloadMetrics("production", "web-frontend", 7*24*time.Hour)
	if metrics == nil {
		t.Fatal("expected non-nil metrics for production/web-frontend")
	}
	if metrics.Name != "web-frontend" {
		t.Errorf("expected name 'web-frontend', got %q", metrics.Name)
	}
	if metrics.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", metrics.Namespace)
	}
	if len(metrics.Containers) == 0 {
		t.Error("expected at least one container in metrics")
	}
	if metrics.StepSeconds != 300 {
		t.Errorf("expected step 300s, got %d", metrics.StepSeconds)
	}
	for _, cm := range metrics.Containers {
		if cm.Name == "" {
			t.Error("container has empty name")
		}
		if len(cm.Series) == 0 {
			t.Errorf("container %q has empty series", cm.Name)
		}
		// Check first point is valid
		if len(cm.Series) > 0 {
			pt := cm.Series[0]
			if pt.Timestamp.IsZero() {
				t.Error("first point has zero timestamp")
			}
			if pt.CPUCores <= 0 {
				t.Errorf("first point has non-positive CPU: %f", pt.CPUCores)
			}
			if pt.MemoryGiB <= 0 {
				t.Errorf("first point has non-positive memory: %f", pt.MemoryGiB)
			}
		}
	}

	// Test with nonexistent workload
	nilMetrics := m.GetWorkloadMetrics("nonexistent", "nope", 7*24*time.Hour)
	if nilMetrics != nil {
		t.Error("expected nil for nonexistent workload")
	}

	// Test determinism: same input → same output
	m2 := NewMockDataProvider(testConfig())
	metrics2 := m2.GetWorkloadMetrics("production", "web-frontend", 7*24*time.Hour)
	if len(metrics.Containers) != len(metrics2.Containers) {
		t.Fatalf("container count mismatch: %d vs %d", len(metrics.Containers), len(metrics2.Containers))
	}
	for i := range metrics.Containers {
		if len(metrics.Containers[i].Series) != len(metrics2.Containers[i].Series) {
			t.Errorf("series length mismatch for container %d", i)
			continue
		}
		if len(metrics.Containers[i].Series) > 0 {
			p1 := metrics.Containers[i].Series[0]
			p2 := metrics2.Containers[i].Series[0]
			if p1.CPUCores != p2.CPUCores {
				t.Errorf("CPU not deterministic: %f vs %f", p1.CPUCores, p2.CPUCores)
			}
		}
	}
}

func TestDeterministic(t *testing.T) {
	cfg := testConfig()
	m1 := NewMockDataProvider(cfg)
	m2 := NewMockDataProvider(cfg)

	w1 := m1.GetAllWorkloads()
	w2 := m2.GetAllWorkloads()

	if len(w1) != len(w2) {
		t.Fatalf("workload counts differ: %d vs %d", len(w1), len(w2))
	}

	for i := range w1 {
		if w1[i].Name != w2[i].Name {
			t.Errorf("workload[%d] name mismatch: %q vs %q", i, w1[i].Name, w2[i].Name)
		}
		if w1[i].Namespace != w2[i].Namespace {
			t.Errorf("workload[%d] namespace mismatch: %q vs %q", i, w1[i].Namespace, w2[i].Namespace)
		}
		if len(w1[i].Containers) != len(w2[i].Containers) {
			t.Errorf("workload[%d] container count mismatch", i)
			continue
		}
		for j := range w1[i].Containers {
			c1 := w1[i].Containers[j]
			c2 := w2[i].Containers[j]
			if c1.Usage.Average.CPUCores != c2.Usage.Average.CPUCores {
				t.Errorf("workload[%d].container[%d] CPU usage mismatch: %f vs %f",
					i, j, c1.Usage.Average.CPUCores, c2.Usage.Average.CPUCores)
			}
			if c1.Usage.Average.MemoryGiB != c2.Usage.Average.MemoryGiB {
				t.Errorf("workload[%d].container[%d] mem usage mismatch: %f vs %f",
					i, j, c1.Usage.Average.MemoryGiB, c2.Usage.Average.MemoryGiB)
			}
		}
	}
}
