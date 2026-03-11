// Package prometheus provides a Prometheus-compatible metrics client.
//
// VictoriaMetrics is fully supported: it implements the same HTTP API as
// Prometheus (including /-/healthy, /api/v1/query, and
// /api/v1/label/<name>/values), so a single client implementation works
// for both backends.  Simply point PROMETHEUS_URL at your VictoriaMetrics
// instance and everything works without any code changes.
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

const bytesPerGiB = 1024 * 1024 * 1024

// Client wraps the Prometheus-compatible HTTP API.
// It works with both Prometheus and VictoriaMetrics.
type Client struct {
	baseURL    string
	httpClient *http.Client
	lookback   time.Duration
}

// NewClient creates a new Prometheus-compatible metrics client.
// baseURL may point to either a Prometheus or VictoriaMetrics instance.
func NewClient(baseURL string, lookback time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		lookback:   lookback,
	}
}

// ── Response types ───────────────────────────────────────────────────────────

// promResponse is the top-level Prometheus HTTP API response envelope.
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promSample `json:"result"`
}

// promSample is a single instant-query result.
// Value is [unix_timestamp, "numeric_string"].
type promSample struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// floatValue returns the numeric value of the sample, or 0 on error/NaN/Inf.
func (s *promSample) floatValue() float64 {
	if len(s.Value) < 2 {
		return 0
	}
	str, ok := s.Value[1].(string)
	if !ok {
		return 0
	}
	v, err := strconv.ParseFloat(str, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

type labelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// ── Core HTTP helpers ────────────────────────────────────────────────────────

// HealthCheck tests connectivity to the metrics backend.
// Both Prometheus and VictoriaMetrics expose /-/healthy.
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/-/healthy", nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("metrics backend health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics backend returned status %d", resp.StatusCode)
	}
	return nil
}

// queryInstant executes a PromQL instant query against /api/v1/query.
func (c *Client) queryInstant(ctx context.Context, query string) ([]promSample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/query", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	q := req.URL.Query()
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query returned HTTP %d: %s", resp.StatusCode, body)
	}

	var result promResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("query status: %s", result.Status)
	}
	return result.Data.Result, nil
}

// ── Public query methods ─────────────────────────────────────────────────────

// GetNamespaces returns all distinct namespace values seen in the metrics backend.
func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/label/namespace/values", nil)
	if err != nil {
		return nil, fmt.Errorf("building namespace request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching namespaces: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("namespace query returned HTTP %d: %s", resp.StatusCode, body)
	}

	var result labelValuesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding namespace response: %w", err)
	}
	return result.Data, nil
}

// GetWorkloads returns analyzed workloads for a single namespace.
func (c *Client) GetWorkloads(ctx context.Context, namespace string) ([]models.RawWorkload, error) {
	all, err := c.GetAllWorkloads(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []models.RawWorkload
	for _, w := range all {
		if w.Namespace == namespace {
			filtered = append(filtered, w)
		}
	}
	return filtered, nil
}

// GetAllWorkloads queries the metrics backend for all workload resource data.
//
// It expects the following metric families to be present, which are provided
// by kube-state-metrics and the kubelet's cAdvisor endpoint:
//
//   - kube_deployment_spec_replicas / kube_statefulset_spec_replicas
//   - kube_pod_owner / kube_replicaset_owner
//   - kube_pod_container_resource_requests / _limits
//   - kube_pod_container_info
//   - container_cpu_usage_seconds_total
//   - container_memory_working_set_bytes
//
// All queries are executed in parallel to minimise total latency.
func (c *Client) GetAllWorkloads(ctx context.Context) ([]models.RawWorkload, error) {
	lb := c.LookbackWindow()

	// All PromQL queries to execute in parallel.
	querySpecs := map[string]string{
		"dep_replicas": "kube_deployment_spec_replicas",
		"ss_replicas":  "kube_statefulset_spec_replicas",
		"pod_to_rs":    `kube_pod_owner{owner_kind="ReplicaSet"}`,
		"rs_to_dep":    `kube_replicaset_owner{owner_kind="Deployment"}`,
		"pod_to_ss":    `kube_pod_owner{owner_kind="StatefulSet"}`,
		"cpu_req":      `kube_pod_container_resource_requests{resource="cpu",container!=""}`,
		"mem_req":      `kube_pod_container_resource_requests{resource="memory",container!=""}`,
		"cpu_lim":      `kube_pod_container_resource_limits{resource="cpu",container!=""}`,
		"mem_lim":      `kube_pod_container_resource_limits{resource="memory",container!=""}`,
		"ctr_info":     `kube_pod_container_info{container!=""}`,
		"cpu_avg":      fmt.Sprintf(`avg_over_time(rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"cpu_p50":      fmt.Sprintf(`quantile_over_time(0.50,rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"cpu_p90":      fmt.Sprintf(`quantile_over_time(0.90,rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"cpu_p95":      fmt.Sprintf(`quantile_over_time(0.95,rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"cpu_p99":      fmt.Sprintf(`quantile_over_time(0.99,rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"cpu_max":      fmt.Sprintf(`max_over_time(rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])[%s:5m])`, lb),
		"mem_avg":      fmt.Sprintf(`avg_over_time(container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
		"mem_p50":      fmt.Sprintf(`quantile_over_time(0.50,container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
		"mem_p90":      fmt.Sprintf(`quantile_over_time(0.90,container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
		"mem_p95":      fmt.Sprintf(`quantile_over_time(0.95,container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
		"mem_p99":      fmt.Sprintf(`quantile_over_time(0.99,container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
		"mem_max":      fmt.Sprintf(`max_over_time(container_memory_working_set_bytes{container!="",container!="POD"}[%s:5m])`, lb),
	}

	type queryResult struct {
		samples []promSample
		err     error
	}

	results := make(map[string]queryResult, len(querySpecs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, query := range querySpecs {
		wg.Add(1)
		go func(name, query string) {
			defer wg.Done()
			samples, err := c.queryInstant(ctx, query)
			mu.Lock()
			results[name] = queryResult{samples, err}
			mu.Unlock()
		}(name, query)
	}
	wg.Wait()

	// Fail fast if workload-structure queries errored — the rest is best-effort.
	for _, name := range []string{"dep_replicas", "ss_replicas", "pod_to_rs", "rs_to_dep", "pod_to_ss"} {
		if e := results[name]; e.err != nil {
			return nil, fmt.Errorf("query %s failed: %w", name, e.err)
		}
	}

	// ── Build pod → workload mapping ─────────────────────────────────────────

	type nsStr struct{ ns, name string }

	// (ns, replicaset) → deployment name
	rsToDeployment := make(map[nsStr]string)
	for _, s := range results["rs_to_dep"].samples {
		ns, rs, dep := s.Metric["namespace"], s.Metric["replicaset"], s.Metric["owner_name"]
		if ns != "" && rs != "" && dep != "" {
			rsToDeployment[nsStr{ns, rs}] = dep
		}
	}

	type podKey struct{ ns, pod string }
	type wlID struct {
		ns    string
		name  string
		wtype models.WorkloadType
	}
	podToWorkload := make(map[podKey]wlID)
	for _, s := range results["pod_to_rs"].samples {
		ns, pod, rs := s.Metric["namespace"], s.Metric["pod"], s.Metric["owner_name"]
		if dep, ok := rsToDeployment[nsStr{ns, rs}]; ok {
			podToWorkload[podKey{ns, pod}] = wlID{ns, dep, models.Deployment}
		}
	}
	for _, s := range results["pod_to_ss"].samples {
		ns, pod, ss := s.Metric["namespace"], s.Metric["pod"], s.Metric["owner_name"]
		if ns != "" && pod != "" && ss != "" {
			podToWorkload[podKey{ns, pod}] = wlID{ns, ss, models.StatefulSet}
		}
	}

	// ── Replica counts ───────────────────────────────────────────────────────

	type wlKey struct {
		ns    string
		name  string
		wtype models.WorkloadType
	}
	replicaM := make(map[wlKey]int)
	for _, s := range results["dep_replicas"].samples {
		ns, dep := s.Metric["namespace"], s.Metric["deployment"]
		if ns != "" && dep != "" {
			replicaM[wlKey{ns, dep, models.Deployment}] = int(s.floatValue())
		}
	}
	for _, s := range results["ss_replicas"].samples {
		ns, ss := s.Metric["namespace"], s.Metric["statefulset"]
		if ns != "" && ss != "" {
			replicaM[wlKey{ns, ss, models.StatefulSet}] = int(s.floatValue())
		}
	}

	// ── Build per-(ns,pod,container) value maps ──────────────────────────────

	type ctrKey struct{ ns, pod, container string }

	buildF := func(name string) map[ctrKey]float64 {
		m := make(map[ctrKey]float64, len(results[name].samples))
		for _, s := range results[name].samples {
			m[ctrKey{s.Metric["namespace"], s.Metric["pod"], s.Metric["container"]}] = s.floatValue()
		}
		return m
	}
	buildGiB := func(name string) map[ctrKey]float64 {
		m := make(map[ctrKey]float64, len(results[name].samples))
		for _, s := range results[name].samples {
			m[ctrKey{s.Metric["namespace"], s.Metric["pod"], s.Metric["container"]}] = s.floatValue() / bytesPerGiB
		}
		return m
	}

	cpuReqM, cpuLimM := buildF("cpu_req"), buildF("cpu_lim")
	cpuAvgM := buildF("cpu_avg")
	cpuP50M, cpuP90M, cpuP95M, cpuP99M, cpuMaxM :=
		buildF("cpu_p50"), buildF("cpu_p90"), buildF("cpu_p95"), buildF("cpu_p99"), buildF("cpu_max")

	memReqM, memLimM := buildGiB("mem_req"), buildGiB("mem_lim")
	memAvgM := buildGiB("mem_avg")
	memP50M, memP90M, memP95M, memP99M, memMaxM :=
		buildGiB("mem_p50"), buildGiB("mem_p90"), buildGiB("mem_p95"), buildGiB("mem_p99"), buildGiB("mem_max")

	imageM := make(map[ctrKey]string)
	for _, s := range results["ctr_info"].samples {
		k := ctrKey{s.Metric["namespace"], s.Metric["pod"], s.Metric["container"]}
		if img := s.Metric["image"]; img != "" && imageM[k] == "" {
			imageM[k] = img
		}
	}

	// ── Aggregate per-pod data per workload ──────────────────────────────────

	type wcKey struct{ ns, workload, container string }
	type aggData struct {
		cpuReq, memReq, cpuLim, memLim                 float64
		cpuAvg, cpuP50, cpuP90, cpuP95, cpuP99, cpuMax float64
		memAvg, memP50, memP90, memP95, memP99, memMax float64
		image                                          string
		n                                              int // pod count
	}

	aggM := make(map[wcKey]*aggData)
	wlTypes := make(map[nsStr]models.WorkloadType) // (ns, workload) → type

	// Collect unique (ns, pod, container) tuples that have any relevant data.
	podCtrs := make(map[ctrKey]struct{})
	for k := range cpuReqM {
		podCtrs[k] = struct{}{}
	}
	for k := range cpuAvgM {
		podCtrs[k] = struct{}{}
	}

	for k := range podCtrs {
		wl, ok := podToWorkload[podKey{k.ns, k.pod}]
		if !ok {
			continue
		}
		wlTypes[nsStr{wl.ns, wl.name}] = wl.wtype

		wck := wcKey{wl.ns, wl.name, k.container}
		a := aggM[wck]
		if a == nil {
			a = &aggData{}
			aggM[wck] = a
		}
		a.cpuReq += cpuReqM[k]
		a.memReq += memReqM[k]
		a.cpuLim += cpuLimM[k]
		a.memLim += memLimM[k]
		a.cpuAvg += cpuAvgM[k]
		a.cpuP50 += cpuP50M[k]
		a.cpuP90 += cpuP90M[k]
		a.cpuP95 += cpuP95M[k]
		a.cpuP99 += cpuP99M[k]
		a.cpuMax += cpuMaxM[k]
		a.memAvg += memAvgM[k]
		a.memP50 += memP50M[k]
		a.memP90 += memP90M[k]
		a.memP95 += memP95M[k]
		a.memP99 += memP99M[k]
		a.memMax += memMaxM[k]
		if a.image == "" {
			a.image = imageM[k]
		}
		a.n++
	}

	// ── Group aggM by workload ───────────────────────────────────────────────

	wlCtrNames := make(map[nsStr][]string) // (ns, workload) → sorted container names
	for wck := range aggM {
		wl := nsStr{wck.ns, wck.workload}
		wlCtrNames[wl] = append(wlCtrNames[wl], wck.container)
	}

	// ── Build final RawWorkload slice ────────────────────────────────────────

	var rawWorkloads []models.RawWorkload
	for wl, ctrs := range wlCtrNames {
		wtype, ok := wlTypes[wl]
		if !ok {
			continue
		}
		reps := replicaM[wlKey{wl.ns, wl.name, wtype}]
		if reps == 0 {
			reps = 1
		}

		sort.Strings(ctrs)
		rawCtrs := make([]models.RawContainer, 0, len(ctrs))
		for _, ctr := range ctrs {
			a := aggM[wcKey{wl.ns, wl.name, ctr}]
			if a == nil || a.n == 0 {
				continue
			}
			n := float64(a.n)
			rawCtrs = append(rawCtrs, models.RawContainer{
				Name:  ctr,
				Image: a.image,
				CurrentRequest: models.ResourceValues{
					CPUCores:  a.cpuReq / n,
					MemoryGiB: a.memReq / n,
				},
				CurrentLimit: models.ResourceValues{
					CPUCores:  a.cpuLim / n,
					MemoryGiB: a.memLim / n,
				},
				Usage: models.UsageStats{
					Average: models.ResourceValues{CPUCores: a.cpuAvg / n, MemoryGiB: a.memAvg / n},
					P50:     models.ResourceValues{CPUCores: a.cpuP50 / n, MemoryGiB: a.memP50 / n},
					P90:     models.ResourceValues{CPUCores: a.cpuP90 / n, MemoryGiB: a.memP90 / n},
					P95:     models.ResourceValues{CPUCores: a.cpuP95 / n, MemoryGiB: a.memP95 / n},
					P99:     models.ResourceValues{CPUCores: a.cpuP99 / n, MemoryGiB: a.memP99 / n},
					Max:     models.ResourceValues{CPUCores: a.cpuMax / n, MemoryGiB: a.memMax / n},
				},
			})
		}
		if len(rawCtrs) == 0 {
			continue
		}
		rawWorkloads = append(rawWorkloads, models.RawWorkload{
			Name:       wl.name,
			Namespace:  wl.ns,
			Type:       wtype,
			Replicas:   reps,
			Containers: rawCtrs,
		})
	}

	sort.Slice(rawWorkloads, func(i, j int) bool {
		if rawWorkloads[i].Namespace != rawWorkloads[j].Namespace {
			return rawWorkloads[i].Namespace < rawWorkloads[j].Namespace
		}
		return rawWorkloads[i].Name < rawWorkloads[j].Name
	})

	return rawWorkloads, nil
}

// LookbackWindow returns the configured lookback as a PromQL duration string.
func (c *Client) LookbackWindow() string {
	hours := int(c.lookback.Hours())
	if hours >= 24 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}

// ── Range query types ────────────────────────────────────────────────────────

type promRangeResponse struct {
	Status string        `json:"status"`
	Data   promRangeData `json:"data"`
}

type promRangeData struct {
	ResultType string            `json:"resultType"`
	Result     []promRangeSeries `json:"result"`
}

type promRangeSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

// queryRange executes a PromQL range query against /api/v1/query_range.
func (c *Client) queryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]promRangeSeries, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/query_range", nil)
	if err != nil {
		return nil, fmt.Errorf("building range request: %w", err)
	}
	q := req.URL.Query()
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", fmt.Sprintf("%ds", int(step.Seconds())))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing range query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading range response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("range query returned HTTP %d: %s", resp.StatusCode, body)
	}

	var result promRangeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding range response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("range query status: %s", result.Status)
	}
	return result.Data.Result, nil
}

// GetWorkloadMetrics queries Prometheus for time-series CPU and memory usage
// for all containers in the given workload over the lookback window.
func (c *Client) GetWorkloadMetrics(ctx context.Context, namespace, name string, lookback time.Duration) (*models.WorkloadMetrics, error) {
	if lookback <= 0 {
		lookback = c.lookback
	}

	step := 5 * time.Minute
	end := time.Now().UTC()
	start := end.Add(-lookback)

	// We query CPU rate and memory usage, filtered by namespace.
	// The workload-to-pod mapping is done via kube_pod_owner.
	cpuQuery := fmt.Sprintf(
		`rate(container_cpu_usage_seconds_total{namespace=%q,container!="",container!="POD"}[5m])`,
		namespace,
	)
	memQuery := fmt.Sprintf(
		`container_memory_working_set_bytes{namespace=%q,container!="",container!="POD"}`,
		namespace,
	)

	type rangeResult struct {
		series []promRangeSeries
		err    error
	}

	var cpuRes, memRes rangeResult
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		s, err := c.queryRange(ctx, cpuQuery, start, end, step)
		cpuRes = rangeResult{s, err}
	}()
	go func() {
		defer wg.Done()
		s, err := c.queryRange(ctx, memQuery, start, end, step)
		memRes = rangeResult{s, err}
	}()
	wg.Wait()

	if cpuRes.err != nil {
		return nil, fmt.Errorf("CPU range query: %w", cpuRes.err)
	}
	if memRes.err != nil {
		return nil, fmt.Errorf("memory range query: %w", memRes.err)
	}

	// Build per-container time-series by matching on pod labels.
	// For simplicity we aggregate across pods belonging to the same container name.
	type tsKey struct {
		container string
		ts        int64
	}
	cpuMap := make(map[tsKey]float64)
	memMap := make(map[tsKey]float64)
	cpuCount := make(map[tsKey]int)
	memCount := make(map[tsKey]int)
	containerSet := make(map[string]bool)

	for _, s := range cpuRes.series {
		ctr := s.Metric["container"]
		if ctr == "" {
			continue
		}
		containerSet[ctr] = true
		for _, v := range s.Values {
			if len(v) < 2 {
				continue
			}
			tsFloat, ok := v[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := v[1].(string)
			if !ok {
				continue
			}
			val, _ := strconv.ParseFloat(valStr, 64)
			k := tsKey{ctr, int64(tsFloat)}
			cpuMap[k] += val
			cpuCount[k]++
		}
	}
	for _, s := range memRes.series {
		ctr := s.Metric["container"]
		if ctr == "" {
			continue
		}
		containerSet[ctr] = true
		for _, v := range s.Values {
			if len(v) < 2 {
				continue
			}
			tsFloat, ok := v[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := v[1].(string)
			if !ok {
				continue
			}
			val, _ := strconv.ParseFloat(valStr, 64)
			k := tsKey{ctr, int64(tsFloat)}
			memMap[k] += val / bytesPerGiB
			memCount[k]++
		}
	}

	if len(containerSet) == 0 {
		return nil, nil
	}

	// Collect all unique timestamps.
	tsSet := make(map[int64]bool)
	for k := range cpuMap {
		tsSet[k.ts] = true
	}
	for k := range memMap {
		tsSet[k.ts] = true
	}
	timestamps := make([]int64, 0, len(tsSet))
	for ts := range tsSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// Build container metrics.
	var cms []models.ContainerMetrics
	for ctr := range containerSet {
		series := make([]models.TimeSeriesPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			k := tsKey{ctr, ts}
			cpu := cpuMap[k]
			if n := cpuCount[k]; n > 1 {
				cpu /= float64(n)
			}
			mem := memMap[k]
			if n := memCount[k]; n > 1 {
				mem /= float64(n)
			}
			series = append(series, models.TimeSeriesPoint{
				Timestamp: time.Unix(ts, 0).UTC(),
				CPUCores:  cpu,
				MemoryGiB: mem,
			})
		}
		cms = append(cms, models.ContainerMetrics{
			Name:   ctr,
			Series: series,
		})
	}
	sort.Slice(cms, func(i, j int) bool { return cms[i].Name < cms[j].Name })

	return &models.WorkloadMetrics{
		Name:        name,
		Namespace:   namespace,
		Containers:  cms,
		StepSeconds: int(step.Seconds()),
	}, nil
}
