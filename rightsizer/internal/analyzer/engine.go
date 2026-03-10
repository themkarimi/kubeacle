package analyzer

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kubeacle/kubeacle/rightsizer/internal/models"
)

// Engine performs rightsizing analysis on Kubernetes workloads.
type Engine struct {
	cfg models.Config
}

// NewEngine creates a new analysis engine with the given configuration.
func NewEngine(cfg models.Config) *Engine {
	return &Engine{cfg: cfg}
}

// AnalyzeWorkload analyzes a single workload and returns recommendations.
func (e *Engine) AnalyzeWorkload(workload models.RawWorkload) models.WorkloadAnalysis {
	containers := make([]models.ContainerAnalysis, 0, len(workload.Containers))
	var totalWaste float64
	highestRisk := models.Low

	for _, c := range workload.Containers {
		ca := e.analyzeContainer(c, workload)
		containers = append(containers, ca)
		totalWaste += ca.WasteScore
		highestRisk = maxRisk(highestRisk, ca.RiskLevel)
	}

	overallWaste := 0.0
	if len(containers) > 0 {
		overallWaste = totalWaste / float64(len(containers))
	}

	return models.WorkloadAnalysis{
		ID:           workload.Namespace + "/" + workload.Name,
		Name:         workload.Name,
		Namespace:    workload.Namespace,
		Type:         workload.Type,
		Replicas:     workload.Replicas,
		QoSClass:     determineQoS(containers),
		Containers:   containers,
		OverallWaste: overallWaste,
		OverallRisk:  highestRisk,
		LastAnalyzed: time.Now(),
	}
}

// ComputeClusterSummary aggregates analysis results across all workloads.
func (e *Engine) ComputeClusterSummary(workloads []models.WorkloadAnalysis) models.ClusterSummary {
	summary := models.ClusterSummary{
		RiskDistribution: make(map[models.RiskLevel]int),
	}

	nsMap := make(map[string]*models.NamespaceSummary)

	for _, w := range workloads {
		summary.TotalWorkloads++
		summary.RiskDistribution[w.OverallRisk]++

		ns, ok := nsMap[w.Namespace]
		if !ok {
			ns = &models.NamespaceSummary{Namespace: w.Namespace}
			nsMap[w.Namespace] = ns
		}
		ns.WorkloadCount++

		for _, c := range w.Containers {
			summary.TotalContainers++
			summary.CPURequestedCores += c.CurrentRequest.CPUCores
			summary.MemRequestedGiB += c.CurrentRequest.MemoryGiB
			summary.CPUUsedCores += c.Usage.Average.CPUCores
			summary.MemUsedGiB += c.Usage.Average.MemoryGiB
			summary.EstimatedMonthlySave += c.Recommended.EstimatedSaving
			ns.EstimatedSaving += c.Recommended.EstimatedSaving
		}
	}

	if summary.CPURequestedCores > 0 {
		summary.CPUWastePercent = ((summary.CPURequestedCores - summary.CPUUsedCores) / summary.CPURequestedCores) * 100
	}
	if summary.MemRequestedGiB > 0 {
		summary.MemWastePercent = ((summary.MemRequestedGiB - summary.MemUsedGiB) / summary.MemRequestedGiB) * 100
	}

	for _, ns := range nsMap {
		var cpuReq, cpuUsed, memReq, memUsed float64
		for _, w := range workloads {
			if w.Namespace != ns.Namespace {
				continue
			}
			for _, c := range w.Containers {
				cpuReq += c.CurrentRequest.CPUCores
				cpuUsed += c.Usage.Average.CPUCores
				memReq += c.CurrentRequest.MemoryGiB
				memUsed += c.Usage.Average.MemoryGiB
			}
		}
		if cpuReq > 0 {
			ns.CPUWastePercent = ((cpuReq - cpuUsed) / cpuReq) * 100
		}
		if memReq > 0 {
			ns.MemWastePercent = ((memReq - memUsed) / memReq) * 100
		}
		summary.NamespaceSummaries = append(summary.NamespaceSummaries, *ns)
	}

	return summary
}

func (e *Engine) analyzeContainer(c models.RawContainer, w models.RawWorkload) models.ContainerAnalysis {
	spike := spikeBaseline(c.Usage, e.cfg.SpikePercentile)
	headroom := e.cfg.HeadroomFactor

	recRequest := models.ResourceValues{
		CPUCores:  spike.CPUCores * (1 + headroom),
		MemoryGiB: spike.MemoryGiB * (1 + headroom),
	}

	recLimit := models.ResourceValues{
		CPUCores:  math.Max(c.Usage.Max.CPUCores*1.3, recRequest.CPUCores*2),
		MemoryGiB: math.Max(c.Usage.Max.MemoryGiB*1.3, recRequest.MemoryGiB*2),
	}

	cpuWaste := wasteScore(c.CurrentRequest.CPUCores, recRequest.CPUCores)
	memWaste := wasteScore(c.CurrentRequest.MemoryGiB, recRequest.MemoryGiB)
	avgWaste := (cpuWaste + memWaste) / 2

	risk := computeRiskLevel(c.CurrentRequest, recRequest)

	issues := detectIssues(c, avgWaste)

	confidence := 0.85

	cpuSaving := (c.CurrentRequest.CPUCores - recRequest.CPUCores) * float64(w.Replicas) * e.cfg.CostPerCPUHour * 730
	memSaving := (c.CurrentRequest.MemoryGiB - recRequest.MemoryGiB) * float64(w.Replicas) * e.cfg.CostPerGiBHour * 730
	totalSaving := cpuSaving + memSaving
	if totalSaving < 0 {
		totalSaving = 0
	}

	reasoning := buildReasoning(c, recRequest, recLimit, avgWaste, risk)
	yamlPatch := generateYAMLPatch(c.Name, recRequest, recLimit)
	kubectlCmd := generateKubectlCmd(w, c.Name, recRequest, recLimit)

	return models.ContainerAnalysis{
		Name:           c.Name,
		Image:          c.Image,
		CurrentRequest: c.CurrentRequest,
		CurrentLimit:   c.CurrentLimit,
		Usage:          c.Usage,
		Recommended: models.Recommendation{
			Request:         recRequest,
			Limit:           recLimit,
			HeadroomFactor:  headroom,
			EstimatedSaving: totalSaving,
			Reasoning:       reasoning,
			YAMLPatch:       yamlPatch,
			KubectlCmd:      kubectlCmd,
		},
		WasteScore:      avgWaste,
		RiskLevel:       risk,
		Issues:          issues,
		ConfidenceScore: confidence,
	}
}

// spikeBaseline selects the appropriate percentile as the spike baseline.
func spikeBaseline(usage models.UsageStats, percentile string) models.ResourceValues {
	switch percentile {
	case "P90":
		return usage.P90
	case "P95":
		return usage.P95
	case "P99":
		return usage.P99
	case "Max":
		return usage.Max
	default:
		return usage.P95
	}
}

// wasteScore calculates how far the current request overshoots the recommended value (0-100).
func wasteScore(current, recommended float64) float64 {
	if current <= 0 {
		return 0
	}
	score := ((current - recommended) / current) * 100
	return math.Max(0, math.Min(100, score))
}

// computeRiskLevel determines risk based on how aggressive the downward change is.
func computeRiskLevel(current, recommended models.ResourceValues) models.RiskLevel {
	cpuRisk := resourceRisk(current.CPUCores, recommended.CPUCores)
	memRisk := resourceRisk(current.MemoryGiB, recommended.MemoryGiB)
	return maxRisk(cpuRisk, memRisk)
}

func resourceRisk(current, recommended float64) models.RiskLevel {
	if current <= 0 {
		return models.Low
	}
	if recommended >= current {
		return models.Low
	}
	reduction := (current - recommended) / current
	switch {
	case reduction < 0.20:
		return models.Low
	case reduction < 0.50:
		return models.Medium
	case reduction < 0.70:
		return models.High
	default:
		return models.Critical
	}
}

// detectIssues identifies potential problems with the container configuration.
func detectIssues(c models.RawContainer, waste float64) []string {
	var issues []string

	if c.CurrentLimit.CPUCores == 0 || c.CurrentLimit.MemoryGiB == 0 {
		issues = append(issues, "no_limits")
	}
	if c.Usage.P95.CPUCores > c.CurrentRequest.CPUCores || c.Usage.P95.MemoryGiB > c.CurrentRequest.MemoryGiB {
		issues = append(issues, "under_provisioned")
	}
	if waste > 50 {
		issues = append(issues, "over_provisioned")
	}
	if c.CurrentLimit.MemoryGiB > 0 && c.Usage.Max.MemoryGiB > c.CurrentLimit.MemoryGiB*0.9 {
		issues = append(issues, "oom_risk")
	}

	return issues
}

func maxRisk(a, b models.RiskLevel) models.RiskLevel {
	order := map[models.RiskLevel]int{
		models.Low:      0,
		models.Medium:   1,
		models.High:     2,
		models.Critical: 3,
	}
	if order[b] > order[a] {
		return b
	}
	return a
}

func determineQoS(containers []models.ContainerAnalysis) models.QoSClass {
	allGuaranteed := true
	for _, c := range containers {
		if c.CurrentRequest.CPUCores != c.CurrentLimit.CPUCores || c.CurrentRequest.MemoryGiB != c.CurrentLimit.MemoryGiB {
			allGuaranteed = false
			break
		}
		if c.CurrentLimit.CPUCores == 0 || c.CurrentLimit.MemoryGiB == 0 {
			allGuaranteed = false
			break
		}
	}
	if allGuaranteed && len(containers) > 0 {
		return models.Guaranteed
	}

	anyRequests := false
	for _, c := range containers {
		if c.CurrentRequest.CPUCores > 0 || c.CurrentRequest.MemoryGiB > 0 {
			anyRequests = true
			break
		}
	}
	if anyRequests {
		return models.Burstable
	}

	return models.BestEffort
}

func buildReasoning(c models.RawContainer, recReq, recLimit models.ResourceValues, waste float64, risk models.RiskLevel) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Container %q analysis:", c.Name))
	parts = append(parts, fmt.Sprintf("  CPU: current request=%.3f cores, recommended=%.3f cores", c.CurrentRequest.CPUCores, recReq.CPUCores))
	parts = append(parts, fmt.Sprintf("  Memory: current request=%.3f GiB, recommended=%.3f GiB", c.CurrentRequest.MemoryGiB, recReq.MemoryGiB))
	parts = append(parts, fmt.Sprintf("  Waste score: %.1f%%, Risk level: %s", waste, risk))
	return strings.Join(parts, "\n")
}

func generateYAMLPatch(containerName string, req, limit models.ResourceValues) string {
	return fmt.Sprintf(`containers:
- name: %s
  resources:
    requests:
      cpu: "%dm"
      memory: "%dMi"
    limits:
      cpu: "%dm"
      memory: "%dMi"`,
		containerName,
		int(math.Ceil(req.CPUCores*1000)),
		int(math.Ceil(req.MemoryGiB*1024)),
		int(math.Ceil(limit.CPUCores*1000)),
		int(math.Ceil(limit.MemoryGiB*1024)),
	)
}

func generateKubectlCmd(w models.RawWorkload, containerName string, req, limit models.ResourceValues) string {
	resourceType := strings.ToLower(string(w.Type))
	return fmt.Sprintf(
		"kubectl set resources %s/%s -c %s --requests=cpu=%dm,memory=%dMi --limits=cpu=%dm,memory=%dMi -n %s",
		resourceType,
		w.Name,
		containerName,
		int(math.Ceil(req.CPUCores*1000)),
		int(math.Ceil(req.MemoryGiB*1024)),
		int(math.Ceil(limit.CPUCores*1000)),
		int(math.Ceil(limit.MemoryGiB*1024)),
		w.Namespace,
	)
}
