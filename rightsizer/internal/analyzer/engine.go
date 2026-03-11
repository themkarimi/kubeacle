package analyzer

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

// Analysis-tuning constants.
const (
	// limitMultiplier is applied to Max usage when computing recommended limits.
	limitMultiplier = 1.3
	// limitRequestRatio is the minimum ratio of limit to request.
	limitRequestRatio = 2.0
	// containerConfidence is the default confidence score for container recommendations.
	containerConfidence = 0.85
	// volumeConfidence is the default confidence score for volume recommendations.
	volumeConfidence = 0.72
	// hoursPerMonth is the average number of hours in a month (365/12*24), used for cost estimation.
	hoursPerMonth = 730
	// minVolumeRequestGiB is the floor for recommended volume sizes.
	minVolumeRequestGiB = 1.0
)

// Risk-threshold constants define the reduction-fraction boundaries for each risk level.
const (
	riskThresholdLow    = 0.20
	riskThresholdMedium = 0.50
	riskThresholdHigh   = 0.70
)

// Issue-detection thresholds.
const (
	overProvisionedWasteThreshold   = 50  // container waste score above which "over_provisioned" is flagged
	oomRiskLimitFraction            = 0.9 // memory usage fraction of limit above which "oom_risk" is flagged
	storageOverProvisionedThreshold = 35  // volume waste score above which "storage_over_provisioned" is flagged
	storageUnderProvisionedFraction = 0.85
	storageCapacityRiskFraction     = 0.95
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
	volumes := make([]models.PersistentVolumeAnalysis, 0, len(workload.PersistentVolumes))
	var totalWaste float64
	var totalStorageWaste float64
	highestRisk := models.Low

	for _, c := range workload.Containers {
		ca := e.analyzeContainer(c, workload)
		containers = append(containers, ca)
		totalWaste += ca.WasteScore
		highestRisk = maxRisk(highestRisk, ca.RiskLevel)
	}

	for _, volume := range workload.PersistentVolumes {
		va := e.analyzeVolume(volume, workload)
		volumes = append(volumes, va)
		totalStorageWaste += va.WasteScore
		highestRisk = maxRisk(highestRisk, va.RiskLevel)
	}

	overallWaste := 0.0
	if len(containers)+len(volumes) > 0 {
		overallWaste = (totalWaste + totalStorageWaste) / float64(len(containers)+len(volumes))
	}

	overallStorageWaste := 0.0
	if len(volumes) > 0 {
		overallStorageWaste = totalStorageWaste / float64(len(volumes))
	}

	return models.WorkloadAnalysis{
		ID:                  workload.Namespace + "/" + workload.Name,
		Name:                workload.Name,
		Namespace:           workload.Namespace,
		Type:                workload.Type,
		Replicas:            workload.Replicas,
		QoSClass:            determineQoS(containers),
		Containers:          containers,
		PersistentVolumes:   volumes,
		OverallWaste:        overallWaste,
		OverallStorageWaste: overallStorageWaste,
		OverallRisk:         highestRisk,
		LastAnalyzed:        time.Now(),
	}
}

// ComputeClusterSummary aggregates analysis results across all workloads.
func (e *Engine) ComputeClusterSummary(workloads []models.WorkloadAnalysis) models.ClusterSummary {
	summary := models.ClusterSummary{
		RiskDistribution: make(map[models.RiskLevel]int),
	}

	type nsAccum struct {
		summary                            *models.NamespaceSummary
		cpuReq, cpuUsed, memReq, memUsed   float64
		storageReq, storageUsed            float64
	}
	nsMap := make(map[string]*nsAccum)

	for _, w := range workloads {
		summary.TotalWorkloads++
		summary.RiskDistribution[w.OverallRisk]++

		acc, ok := nsMap[w.Namespace]
		if !ok {
			acc = &nsAccum{summary: &models.NamespaceSummary{Namespace: w.Namespace}}
			nsMap[w.Namespace] = acc
		}
		acc.summary.WorkloadCount++

		for _, c := range w.Containers {
			summary.TotalContainers++
			summary.CPURequestedCores += c.CurrentRequest.CPUCores
			summary.MemRequestedGiB += c.CurrentRequest.MemoryGiB
			summary.CPUUsedCores += c.Usage.Average.CPUCores
			summary.MemUsedGiB += c.Usage.Average.MemoryGiB
			summary.EstimatedMonthlySave += c.Recommended.EstimatedSaving
			acc.summary.EstimatedSaving += c.Recommended.EstimatedSaving

			acc.cpuReq += c.CurrentRequest.CPUCores
			acc.cpuUsed += c.Usage.Average.CPUCores
			acc.memReq += c.CurrentRequest.MemoryGiB
			acc.memUsed += c.Usage.Average.MemoryGiB
		}

		for _, volume := range w.PersistentVolumes {
			summary.TotalPersistentVolumes++
			summary.StorageRequestedGiB += volume.CurrentRequestGiB
			summary.StorageUsedGiB += volume.Usage.AverageGiB
			summary.EstimatedStorageReclaimGiB += volume.Recommended.ReclaimableGiB

			acc.storageReq += volume.CurrentRequestGiB
			acc.storageUsed += volume.Usage.AverageGiB
		}
	}

	if summary.CPURequestedCores > 0 {
		summary.CPUWastePercent = ((summary.CPURequestedCores - summary.CPUUsedCores) / summary.CPURequestedCores) * 100
	}
	if summary.MemRequestedGiB > 0 {
		summary.MemWastePercent = ((summary.MemRequestedGiB - summary.MemUsedGiB) / summary.MemRequestedGiB) * 100
	}
	if summary.StorageRequestedGiB > 0 {
		summary.StorageWastePercent = ((summary.StorageRequestedGiB - summary.StorageUsedGiB) / summary.StorageRequestedGiB) * 100
	}

	for _, acc := range nsMap {
		ns := acc.summary
		if acc.cpuReq > 0 {
			ns.CPUWastePercent = ((acc.cpuReq - acc.cpuUsed) / acc.cpuReq) * 100
		}
		if acc.memReq > 0 {
			ns.MemWastePercent = ((acc.memReq - acc.memUsed) / acc.memReq) * 100
		}
		if acc.storageReq > 0 {
			ns.StorageWastePercent = ((acc.storageReq - acc.storageUsed) / acc.storageReq) * 100
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
		CPUCores:  math.Max(c.Usage.Max.CPUCores*limitMultiplier, recRequest.CPUCores*limitRequestRatio),
		MemoryGiB: math.Max(c.Usage.Max.MemoryGiB*limitMultiplier, recRequest.MemoryGiB*limitRequestRatio),
	}

	cpuWaste := wasteScore(c.CurrentRequest.CPUCores, recRequest.CPUCores)
	memWaste := wasteScore(c.CurrentRequest.MemoryGiB, recRequest.MemoryGiB)
	avgWaste := (cpuWaste + memWaste) / 2

	risk := computeRiskLevel(c.CurrentRequest, recRequest)

	issues := detectIssues(c, avgWaste)

	confidence := containerConfidence

	cpuSaving := (c.CurrentRequest.CPUCores - recRequest.CPUCores) * float64(w.Replicas) * e.cfg.CostPerCPUHour * hoursPerMonth
	memSaving := (c.CurrentRequest.MemoryGiB - recRequest.MemoryGiB) * float64(w.Replicas) * e.cfg.CostPerGiBHour * hoursPerMonth
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

func (e *Engine) analyzeVolume(volume models.RawPersistentVolume, w models.RawWorkload) models.PersistentVolumeAnalysis {
	spikeGiB := volumeSpikeBaseline(volume.Usage, e.cfg.SpikePercentile)
	headroom := e.cfg.HeadroomFactor
	recommendedGiB := math.Max(spikeGiB*(1+headroom), minVolumeRequestGiB)

	waste := wasteScore(volume.CurrentRequestGiB, recommendedGiB)
	risk := resourceRisk(volume.CurrentRequestGiB, recommendedGiB)
	reclaimableGiB := math.Max(volume.CurrentRequestGiB-recommendedGiB, 0)
	issues := detectVolumeIssues(volume, waste)

	return models.PersistentVolumeAnalysis{
		Name:              volume.Name,
		StorageClass:      volume.StorageClass,
		CurrentRequestGiB: volume.CurrentRequestGiB,
		Usage:             volume.Usage,
		Recommended: models.VolumeRecommendation{
			RequestGiB:     recommendedGiB,
			HeadroomFactor: headroom,
			ReclaimableGiB: reclaimableGiB,
			Reasoning:      buildVolumeReasoning(volume, recommendedGiB, waste, risk),
			YAMLPatch:      generatePVCYAMLPatch(volume.Name, recommendedGiB),
			KubectlCmd:     generatePVCKubectlCmd(w, volume.Name, recommendedGiB),
		},
		WasteScore:      waste,
		RiskLevel:       risk,
		Issues:          issues,
		ConfidenceScore: volumeConfidence,
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

func volumeSpikeBaseline(usage models.VolumeUsageStats, percentile string) float64 {
	switch percentile {
	case "P90":
		return usage.P90GiB
	case "P95":
		return usage.P95GiB
	case "P99":
		return usage.P99GiB
	case "Max":
		return usage.MaxGiB
	default:
		return usage.P95GiB
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
	case reduction < riskThresholdLow:
		return models.Low
	case reduction < riskThresholdMedium:
		return models.Medium
	case reduction < riskThresholdHigh:
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
	if waste > overProvisionedWasteThreshold {
		issues = append(issues, "over_provisioned")
	}
	if c.CurrentLimit.MemoryGiB > 0 && c.Usage.Max.MemoryGiB > c.CurrentLimit.MemoryGiB*oomRiskLimitFraction {
		issues = append(issues, "oom_risk")
	}

	return issues
}

func detectVolumeIssues(volume models.RawPersistentVolume, waste float64) []string {
	var issues []string

	if waste > storageOverProvisionedThreshold {
		issues = append(issues, "storage_over_provisioned")
	}
	if volume.Usage.P95GiB > volume.CurrentRequestGiB*storageUnderProvisionedFraction {
		issues = append(issues, "storage_under_provisioned")
	}
	if volume.Usage.MaxGiB > volume.CurrentRequestGiB*storageCapacityRiskFraction {
		issues = append(issues, "storage_capacity_risk")
	}

	return issues
}

func maxRisk(a, b models.RiskLevel) models.RiskLevel {
	if models.RiskOrder(b) > models.RiskOrder(a) {
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

func buildVolumeReasoning(volume models.RawPersistentVolume, recommendedGiB, waste float64, risk models.RiskLevel) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("PersistentVolumeClaim %q analysis:", volume.Name))
	parts = append(parts, fmt.Sprintf("  Storage: current request=%.2f GiB, recommended=%.2f GiB", volume.CurrentRequestGiB, recommendedGiB))
	parts = append(parts, fmt.Sprintf("  Observed usage: avg=%.2f GiB, p95=%.2f GiB, max=%.2f GiB", volume.Usage.AverageGiB, volume.Usage.P95GiB, volume.Usage.MaxGiB))
	parts = append(parts, fmt.Sprintf("  Waste score: %.1f%%, Risk level: %s", waste, risk))
	parts = append(parts, "  Note: shrinking an existing PVC may require storage-class support or migration to a new claim.")
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

func generatePVCYAMLPatch(pvcName string, requestGiB float64) string {
	return fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
spec:
  resources:
    requests:
      storage: "%dGi"`, pvcName, int(math.Ceil(requestGiB)))
}

func generatePVCKubectlCmd(w models.RawWorkload, pvcName string, requestGiB float64) string {
	return fmt.Sprintf(
		"kubectl patch pvc %s -n %s --type merge -p '{\"spec\":{\"resources\":{\"requests\":{\"storage\":\"%dGi\"}}}}'",
		pvcName,
		w.Namespace,
		int(math.Ceil(requestGiB)),
	)
}
