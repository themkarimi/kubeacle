package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/themkarimi/kubeacle/rightsizer/internal/analyzer"
	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
	"github.com/themkarimi/kubeacle/rightsizer/internal/prometheus"
)

// AnalysisCache is a simple in-memory cache with TTL support.
type AnalysisCache struct {
	data sync.Map
	ttl  time.Duration
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

func NewAnalysisCache(ttl time.Duration) *AnalysisCache {
	return &AnalysisCache{ttl: ttl}
}

func (c *AnalysisCache) Get(key string) (interface{}, bool) {
	raw, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := raw.(cacheEntry)
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.data.Delete(key)
		return nil, false
	}
	return entry.value, true
}

func (c *AnalysisCache) Set(key string, value interface{}) {
	c.data.Store(key, cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an ErrorResponse with the given status code.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg, Code: code})
}

// parsePagination extracts page and page_size from query params with defaults.
func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}
	return
}

// paginate returns a slice of items for the given page.
func paginate[T any](items []T, page, pageSize int) []T {
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	mode := "live"
	if s.cfg.MockMode {
		mode = "mock"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "mode": mode})
}

func (s *Server) handleGetNamespaces(w http.ResponseWriter, r *http.Request) {
	var namespaces []string

	if s.cfg.MockMode {
		namespaces = s.mockData.GetNamespaces()
	} else {
		var err error
		namespaces, err = s.promClient.GetNamespaces(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "PROMETHEUS_ERROR", err.Error())
			return
		}
	}

	var filtered []string
	for _, ns := range namespaces {
		if !s.isExcludedNamespace(ns) {
			filtered = append(filtered, ns)
		}
	}

	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleGetWorkloads(w http.ResponseWriter, r *http.Request) {
	analyzed, err := s.getAnalyzedWorkloads(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANALYSIS_ERROR", err.Error())
		return
	}

	nsFilter := r.URL.Query().Get("namespace")
	typeFilter := r.URL.Query().Get("type")

	var summaries []models.WorkloadSummary
	for _, wa := range analyzed {
		if nsFilter != "" && wa.Namespace != nsFilter {
			continue
		}
		if typeFilter != "" && string(wa.Type) != typeFilter {
			continue
		}

		var totalSaving float64
		for _, c := range wa.Containers {
			totalSaving += c.Recommended.EstimatedSaving
		}

		summaries = append(summaries, models.WorkloadSummary{
			ID:              wa.ID,
			Name:            wa.Name,
			Namespace:       wa.Namespace,
			Type:            wa.Type,
			Replicas:        wa.Replicas,
			QoSClass:        wa.QoSClass,
			OverallWaste:    wa.OverallWaste,
			OverallRisk:     wa.OverallRisk,
			Containers:      len(wa.Containers),
			EstimatedSaving: totalSaving,
		})
	}

	page, pageSize := parsePagination(r)
	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:     paginate(summaries, page, pageSize),
		Total:    len(summaries),
		Page:     page,
		PageSize: pageSize,
	})
}

func (s *Server) handleGetWorkloadAnalysis(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	targetID := namespace + "/" + name

	analyzed, err := s.getAnalyzedWorkloads(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANALYSIS_ERROR", err.Error())
		return
	}

	for _, wa := range analyzed {
		if wa.ID == targetID {
			writeJSON(w, http.StatusOK, wa)
			return
		}
	}

	writeError(w, http.StatusNotFound, "NOT_FOUND",
		fmt.Sprintf("workload %s not found", targetID))
}

func (s *Server) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	analyzed, err := s.getAnalyzedWorkloads(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANALYSIS_ERROR", err.Error())
		return
	}

	nsFilter := r.URL.Query().Get("namespace")
	riskFilter := r.URL.Query().Get("risk")
	sortBy := r.URL.Query().Get("sort")

	var filtered []models.WorkloadAnalysis
	for _, wa := range analyzed {
		if nsFilter != "" && wa.Namespace != nsFilter {
			continue
		}
		if riskFilter != "" && string(wa.OverallRisk) != strings.ToUpper(riskFilter) {
			continue
		}
		filtered = append(filtered, wa)
	}

	switch sortBy {
	case "savings":
		sort.Slice(filtered, func(i, j int) bool {
			return totalSaving(filtered[i]) > totalSaving(filtered[j])
		})
	case "waste":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].OverallWaste > filtered[j].OverallWaste
		})
	case "risk":
		riskOrder := map[models.RiskLevel]int{
			models.Critical: 3, models.High: 2, models.Medium: 1, models.Low: 0,
		}
		sort.Slice(filtered, func(i, j int) bool {
			return riskOrder[filtered[i].OverallRisk] > riskOrder[filtered[j].OverallRisk]
		})
	}

	page, pageSize := parsePagination(r)
	writeJSON(w, http.StatusOK, models.PaginatedResponse{
		Data:     paginate(filtered, page, pageSize),
		Total:    len(filtered),
		Page:     page,
		PageSize: pageSize,
	})
}

func (s *Server) handleGetClusterSummary(w http.ResponseWriter, r *http.Request) {
	analyzed, err := s.getAnalyzedWorkloads(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANALYSIS_ERROR", err.Error())
		return
	}

	summary := s.engine.ComputeClusterSummary(analyzed)
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	sanitized := struct {
		PrometheusURL     string        `json:"prometheus_url"`
		LookbackWindow    time.Duration `json:"lookback_window"`
		HeadroomFactor    float64       `json:"headroom_factor"`
		SpikePercentile   string        `json:"spike_percentile"`
		CostPerCPUHour    float64       `json:"cost_per_cpu_hour"`
		CostPerGiBHour    float64       `json:"cost_per_gib_hour"`
		ExcludeNamespaces []string      `json:"exclude_namespaces"`
		MockMode          bool          `json:"mock_mode"`
		Port              int           `json:"port"`
		RefreshInterval   time.Duration `json:"refresh_interval"`
	}{
		PrometheusURL:     s.cfg.PrometheusURL,
		LookbackWindow:    s.cfg.LookbackWindow,
		HeadroomFactor:    s.cfg.HeadroomFactor,
		SpikePercentile:   s.cfg.SpikePercentile,
		CostPerCPUHour:    s.cfg.CostPerCPUHour,
		CostPerGiBHour:    s.cfg.CostPerGiBHour,
		ExcludeNamespaces: s.cfg.ExcludeNamespaces,
		MockMode:          s.cfg.MockMode,
		Port:              s.cfg.Port,
		RefreshInterval:   s.cfg.RefreshInterval,
	}
	writeJSON(w, http.StatusOK, sanitized)
}

type configUpdate struct {
	HeadroomFactor    *float64 `json:"headroom_factor"`
	SpikePercentile   *string  `json:"spike_percentile"`
	CostPerCPUHour    *float64 `json:"cost_per_cpu_hour"`
	CostPerGiBHour    *float64 `json:"cost_per_gib_hour"`
	ExcludeNamespaces []string `json:"exclude_namespaces"`
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var update configUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid JSON body")
		return
	}

	if update.HeadroomFactor != nil {
		s.cfg.HeadroomFactor = *update.HeadroomFactor
	}
	if update.SpikePercentile != nil {
		valid := map[string]bool{"P90": true, "P95": true, "P99": true, "Max": true}
		if !valid[*update.SpikePercentile] {
			writeError(w, http.StatusBadRequest, "INVALID_VALUE",
				"spike_percentile must be one of: P90, P95, P99, Max")
			return
		}
		s.cfg.SpikePercentile = *update.SpikePercentile
	}
	if update.CostPerCPUHour != nil {
		s.cfg.CostPerCPUHour = *update.CostPerCPUHour
	}
	if update.CostPerGiBHour != nil {
		s.cfg.CostPerGiBHour = *update.CostPerGiBHour
	}
	if update.ExcludeNamespaces != nil {
		s.cfg.ExcludeNamespaces = update.ExcludeNamespaces
	}

	// Rebuild the engine with updated config and clear the cache.
	s.engine = analyzer.NewEngine(s.cfg)
	s.cache = NewAnalysisCache(s.cfg.RefreshInterval)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleTestPrometheus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid JSON body")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "url must not be empty")
		return
	}

	testClient := prometheus.NewClient(req.URL, s.cfg.LookbackWindow)
	if err := testClient.HealthCheck(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, "PROMETHEUS_UNHEALTHY", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "url": req.URL})
}

func (s *Server) handlePrometheusHealth(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MockMode {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"mode":   "mock",
			"note":   "prometheus not used in mock mode",
		})
		return
	}

	if err := s.promClient.HealthCheck(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, "PROMETHEUS_UNHEALTHY", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Internal helpers ────────────────────────────────────────────────────────

// getAnalyzedWorkloads retrieves all workloads, runs analysis, and caches the result.
func (s *Server) getAnalyzedWorkloads(ctx context.Context) ([]models.WorkloadAnalysis, error) {
	const cacheKey = "all_workloads"

	if cached, ok := s.cache.Get(cacheKey); ok {
		result, valid := cached.([]models.WorkloadAnalysis)
		if !valid {
			return nil, fmt.Errorf("invalid cache entry type")
		}
		return result, nil
	}

	var rawWorkloads []models.RawWorkload

	if s.cfg.MockMode {
		rawWorkloads = s.mockData.GetAllWorkloads()
	} else {
		var err error
		rawWorkloads, err = s.promClient.GetAllWorkloads(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetching workloads: %w", err)
		}
	}

	analyzed := make([]models.WorkloadAnalysis, 0, len(rawWorkloads))
	for _, rw := range rawWorkloads {
		if s.isExcludedNamespace(rw.Namespace) {
			continue
		}
		analyzed = append(analyzed, s.engine.AnalyzeWorkload(rw))
	}

	s.cache.Set(cacheKey, analyzed)
	return analyzed, nil
}

// totalSaving sums the estimated savings across all containers in a workload.
func totalSaving(wa models.WorkloadAnalysis) float64 {
	var total float64
	for _, c := range wa.Containers {
		total += c.Recommended.EstimatedSaving
	}
	return total
}

// isExcludedNamespace checks if a namespace is in the exclusion list.
func (s *Server) isExcludedNamespace(ns string) bool {
	for _, excluded := range s.cfg.ExcludeNamespaces {
		if strings.EqualFold(strings.TrimSpace(excluded), strings.TrimSpace(ns)) {
			return true
		}
	}
	return false
}
