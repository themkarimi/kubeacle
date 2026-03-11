package models

import "time"

type WorkloadType string

const (
	Deployment  WorkloadType = "Deployment"
	StatefulSet WorkloadType = "StatefulSet"
)

type QoSClass string

const (
	Guaranteed QoSClass = "Guaranteed"
	Burstable  QoSClass = "Burstable"
	BestEffort QoSClass = "BestEffort"
)

type RiskLevel string

const (
	Low      RiskLevel = "LOW"
	Medium   RiskLevel = "MEDIUM"
	High     RiskLevel = "HIGH"
	Critical RiskLevel = "CRITICAL"
)

type ResourceValues struct {
	CPUCores  float64 `json:"cpu_cores"`
	MemoryGiB float64 `json:"memory_gib"`
}

type UsageStats struct {
	P50     ResourceValues `json:"p50"`
	P90     ResourceValues `json:"p90"`
	P95     ResourceValues `json:"p95"`
	P99     ResourceValues `json:"p99"`
	Max     ResourceValues `json:"max"`
	Average ResourceValues `json:"average"`
}

type ContainerAnalysis struct {
	Name            string         `json:"name"`
	Image           string         `json:"image"`
	CurrentRequest  ResourceValues `json:"current_request"`
	CurrentLimit    ResourceValues `json:"current_limit"`
	Usage           UsageStats     `json:"usage"`
	Recommended     Recommendation `json:"recommendation"`
	WasteScore      float64        `json:"waste_score"`
	RiskLevel       RiskLevel      `json:"risk_level"`
	Issues          []string       `json:"issues"`
	ConfidenceScore float64        `json:"confidence_score"`
}

type Recommendation struct {
	Request         ResourceValues `json:"request"`
	Limit           ResourceValues `json:"limit"`
	HeadroomFactor  float64        `json:"headroom_factor"`
	EstimatedSaving float64        `json:"estimated_monthly_saving_usd"`
	Reasoning       string         `json:"reasoning"`
	YAMLPatch       string         `json:"yaml_patch"`
	KubectlCmd      string         `json:"kubectl_cmd"`
}

type WorkloadAnalysis struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Namespace    string              `json:"namespace"`
	Type         WorkloadType        `json:"type"`
	Replicas     int                 `json:"replicas"`
	QoSClass     QoSClass            `json:"qos_class"`
	Containers   []ContainerAnalysis `json:"containers"`
	OverallWaste float64             `json:"overall_waste_score"`
	OverallRisk  RiskLevel           `json:"overall_risk"`
	LastAnalyzed time.Time           `json:"last_analyzed"`
}

type NamespaceSummary struct {
	Namespace       string  `json:"namespace"`
	WorkloadCount   int     `json:"workload_count"`
	CPUWastePercent float64 `json:"cpu_waste_percent"`
	MemWastePercent float64 `json:"mem_waste_percent"`
	EstimatedSaving float64 `json:"estimated_monthly_saving_usd"`
}

type ClusterSummary struct {
	TotalWorkloads       int                `json:"total_workloads"`
	TotalContainers      int                `json:"total_containers"`
	CPURequestedCores    float64            `json:"cpu_requested_cores"`
	CPUUsedCores         float64            `json:"cpu_used_cores"`
	CPUWastePercent      float64            `json:"cpu_waste_percent"`
	MemRequestedGiB      float64            `json:"mem_requested_gib"`
	MemUsedGiB           float64            `json:"mem_used_gib"`
	MemWastePercent      float64            `json:"mem_waste_percent"`
	EstimatedMonthlySave float64            `json:"estimated_monthly_saving_usd"`
	RiskDistribution     map[RiskLevel]int  `json:"risk_distribution"`
	NamespaceSummaries   []NamespaceSummary `json:"namespace_summaries"`
}

type Config struct {
	PrometheusURL     string        `json:"prometheus_url" yaml:"prometheus_url"`
	LookbackWindow    time.Duration `json:"lookback_window" yaml:"lookback_window"`
	HeadroomFactor    float64       `json:"headroom_factor" yaml:"headroom_factor"`
	SpikePercentile   string        `json:"spike_percentile" yaml:"spike_percentile"`
	CostPerCPUHour    float64       `json:"cost_per_cpu_hour" yaml:"cost_per_cpu_hour"`
	CostPerGiBHour    float64       `json:"cost_per_gib_hour" yaml:"cost_per_gib_hour"`
	ExcludeNamespaces []string      `json:"exclude_namespaces" yaml:"exclude_namespaces"`
	MockMode          bool          `json:"mock_mode" yaml:"mock_mode"`
	Port              int           `json:"port" yaml:"port"`
	RefreshInterval   time.Duration `json:"refresh_interval" yaml:"refresh_interval"`
}

// RawWorkload is the raw data from Kubernetes/Prometheus before analysis
type RawWorkload struct {
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace"`
	Type       WorkloadType   `json:"type"`
	Replicas   int            `json:"replicas"`
	Containers []RawContainer `json:"containers"`
}

type RawContainer struct {
	Name           string         `json:"name"`
	Image          string         `json:"image"`
	CurrentRequest ResourceValues `json:"current_request"`
	CurrentLimit   ResourceValues `json:"current_limit"`
	Usage          UsageStats     `json:"usage"`
}

// TimeSeriesPoint is a single timestamped resource measurement.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CPUCores  float64   `json:"cpu_cores"`
	MemoryGiB float64   `json:"memory_gib"`
}

// ContainerMetrics holds time-series data for a single container.
type ContainerMetrics struct {
	Name   string            `json:"name"`
	Series []TimeSeriesPoint `json:"series"`
}

// WorkloadMetrics holds time-series data for all containers in a workload.
type WorkloadMetrics struct {
	Name       string             `json:"name"`
	Namespace  string             `json:"namespace"`
	Containers []ContainerMetrics `json:"containers"`
	StepSeconds int               `json:"step_seconds"`
}

// PaginatedResponse is a generic paginated response
type PaginatedResponse struct {
	Data     interface{} `json:"data"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// WorkloadSummary is a summary view of a workload for list views
type WorkloadSummary struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Namespace       string       `json:"namespace"`
	Type            WorkloadType `json:"type"`
	Replicas        int          `json:"replicas"`
	QoSClass        QoSClass     `json:"qos_class"`
	OverallWaste    float64      `json:"overall_waste_score"`
	OverallRisk     RiskLevel    `json:"overall_risk"`
	Containers      int          `json:"containers"`
	EstimatedSaving float64      `json:"estimated_monthly_saving_usd"`
}
