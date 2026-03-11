package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

func testServer() *Server {
	cfg := models.Config{
		HeadroomFactor:  0.20,
		SpikePercentile: "P95",
		CostPerCPUHour:  0.032,
		CostPerGiBHour:  0.004,
		MockMode:        true,
		Port:            8080,
		RefreshInterval: 5 * time.Minute,
	}
	return NewServer(cfg)
}

func doGet(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestHealthEndpoint(t *testing.T) {
	srv := testServer()
	rec := doGet(t, srv, "/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
	if body["mode"] != "mock" {
		t.Errorf("expected mode 'mock', got %q", body["mode"])
	}
}

func TestGetNamespaces(t *testing.T) {
	srv := testServer()
	rec := doGet(t, srv, "/api/v1/namespaces")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var namespaces []string
	if err := json.NewDecoder(rec.Body).Decode(&namespaces); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(namespaces) == 0 {
		t.Error("expected at least one namespace")
	}

	found := false
	for _, ns := range namespaces {
		if ns == "production" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'production' namespace in the list")
	}
}

func TestGetWorkloads(t *testing.T) {
	srv := testServer()
	rec := doGet(t, srv, "/api/v1/workloads?page=1&page_size=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp models.PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total == 0 {
		t.Error("expected total > 0")
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", resp.PageSize)
	}

	items, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected Data to be an array")
	}
	if len(items) == 0 {
		t.Error("expected at least one workload in data")
	}
	if len(items) > 10 {
		t.Errorf("expected at most 10 items per page, got %d", len(items))
	}
}

func TestGetClusterSummary(t *testing.T) {
	srv := testServer()
	rec := doGet(t, srv, "/api/v1/cluster/summary")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var summary models.ClusterSummary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if summary.TotalWorkloads == 0 {
		t.Error("expected TotalWorkloads > 0")
	}
	if summary.TotalContainers == 0 {
		t.Error("expected TotalContainers > 0")
	}
	if summary.TotalPersistentVolumes == 0 {
		t.Error("expected TotalPersistentVolumes > 0")
	}
	if summary.CPURequestedCores <= 0 {
		t.Error("expected CPURequestedCores > 0")
	}
	if summary.MemRequestedGiB <= 0 {
		t.Error("expected MemRequestedGiB > 0")
	}
	if summary.StorageRequestedGiB <= 0 {
		t.Error("expected StorageRequestedGiB > 0")
	}
	if summary.EstimatedStorageReclaimGiB <= 0 {
		t.Error("expected EstimatedStorageReclaimGiB > 0")
	}
	if summary.RiskDistribution == nil {
		t.Error("expected non-nil RiskDistribution")
	}
	if len(summary.NamespaceSummaries) == 0 {
		t.Error("expected at least one namespace summary")
	}
}

func TestGetWorkloadAnalysis(t *testing.T) {
	srv := testServer()

	// Use a known workload from mock data with PVC analysis: production/postgres-primary
	rec := doGet(t, srv, "/api/v1/workloads/production/postgres-primary/analysis")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var analysis models.WorkloadAnalysis
	if err := json.NewDecoder(rec.Body).Decode(&analysis); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if analysis.ID != "production/postgres-primary" {
		t.Errorf("expected ID 'production/postgres-primary', got %q", analysis.ID)
	}
	if analysis.Name != "postgres-primary" {
		t.Errorf("expected Name 'postgres-primary', got %q", analysis.Name)
	}
	if analysis.Namespace != "production" {
		t.Errorf("expected Namespace 'production', got %q", analysis.Namespace)
	}
	if len(analysis.Containers) == 0 {
		t.Error("expected at least one container in analysis")
	}
	if len(analysis.PersistentVolumes) == 0 {
		t.Error("expected persistent volume analysis for mock workload")
	}

	// Test 404 for nonexistent workload
	rec404 := doGet(t, srv, "/api/v1/workloads/default/nonexistent/analysis")
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent workload, got %d", rec404.Code)
	}
}

func TestGetWorkloadMetrics(t *testing.T) {
	srv := testServer()

	// Use a known workload from mock data: production/web-frontend
	rec := doGet(t, srv, "/api/v1/workloads/production/web-frontend/metrics")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var metrics models.WorkloadMetrics
	if err := json.NewDecoder(rec.Body).Decode(&metrics); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if metrics.Name != "web-frontend" {
		t.Errorf("expected Name 'web-frontend', got %q", metrics.Name)
	}
	if metrics.Namespace != "production" {
		t.Errorf("expected Namespace 'production', got %q", metrics.Namespace)
	}
	if len(metrics.Containers) == 0 {
		t.Error("expected at least one container in metrics")
	}
	if metrics.StepSeconds <= 0 {
		t.Errorf("expected positive StepSeconds, got %d", metrics.StepSeconds)
	}

	// Verify each container has time-series data
	for _, cm := range metrics.Containers {
		if cm.Name == "" {
			t.Error("container metrics has empty name")
		}
		if len(cm.Series) == 0 {
			t.Errorf("container %q has no time-series points", cm.Name)
		}
		// Verify data points have non-zero timestamps and values
		for i, pt := range cm.Series {
			if pt.Timestamp.IsZero() {
				t.Errorf("container %q point %d has zero timestamp", cm.Name, i)
			}
			if pt.CPUCores <= 0 {
				t.Errorf("container %q point %d has non-positive CPU: %f", cm.Name, i, pt.CPUCores)
			}
			if pt.MemoryGiB <= 0 {
				t.Errorf("container %q point %d has non-positive memory: %f", cm.Name, i, pt.MemoryGiB)
			}
			if i > 0 {
				break // Only check first two points for efficiency
			}
		}
	}

	// Test 404 for nonexistent workload
	rec404 := doGet(t, srv, "/api/v1/workloads/default/nonexistent/metrics")
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent workload, got %d", rec404.Code)
	}
}

func TestGetConfig(t *testing.T) {
	srv := testServer()
	rec := doGet(t, srv, "/api/v1/config")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var cfg map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if cfg["mock_mode"] != true {
		t.Errorf("expected mock_mode true, got %v", cfg["mock_mode"])
	}
	if cfg["headroom_factor"] != 0.20 {
		t.Errorf("expected headroom_factor 0.20, got %v", cfg["headroom_factor"])
	}
	if cfg["spike_percentile"] != "P95" {
		t.Errorf("expected spike_percentile 'P95', got %v", cfg["spike_percentile"])
	}
}

func TestTestPrometheusEndpoint(t *testing.T) {
	srv := testServer()

	// Test with empty body
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/test-prometheus",
		strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty url, got %d", rec.Code)
	}

	// Test with invalid URL (connection refused)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/test-prometheus",
		strings.NewReader(`{"url":"http://localhost:19999"}`))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unreachable prometheus, got %d", rec.Code)
	}

	// Test with invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/v1/config/test-prometheus",
		strings.NewReader(`not json`))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestNamespaceExclusion(t *testing.T) {
	cfg := models.Config{
		HeadroomFactor:    0.20,
		SpikePercentile:   "P95",
		CostPerCPUHour:    0.032,
		CostPerGiBHour:    0.004,
		MockMode:          true,
		Port:              8080,
		RefreshInterval:   5 * time.Minute,
		ExcludeNamespaces: []string{"kube-system", "monitoring"},
	}
	srv := NewServer(cfg)

	// Test namespaces endpoint excludes configured namespaces
	rec := doGet(t, srv, "/api/v1/namespaces")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var namespaces []string
	if err := json.NewDecoder(rec.Body).Decode(&namespaces); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	for _, ns := range namespaces {
		if ns == "kube-system" || ns == "monitoring" {
			t.Errorf("namespace %q should be excluded but was returned", ns)
		}
	}
	if len(namespaces) == 0 {
		t.Error("expected at least one namespace after exclusion")
	}

	// Test workloads endpoint excludes configured namespaces
	rec = doGet(t, srv, "/api/v1/workloads?page_size=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	items, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected Data to be an array")
	}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if ns, ok := m["namespace"].(string); ok {
			if ns == "kube-system" || ns == "monitoring" {
				t.Errorf("workload in excluded namespace %q should not appear", ns)
			}
		}
	}

	// Test cluster summary doesn't include excluded namespaces
	rec = doGet(t, srv, "/api/v1/cluster/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var summary models.ClusterSummary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	for _, nsSummary := range summary.NamespaceSummaries {
		if nsSummary.Namespace == "kube-system" || nsSummary.Namespace == "monitoring" {
			t.Errorf("excluded namespace %q should not appear in summary", nsSummary.Namespace)
		}
	}
}
