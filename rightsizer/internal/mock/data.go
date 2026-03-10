package mock

import (
	"math/rand"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

// MockDataProvider generates deterministic fake workload data for development
// and testing when MOCK_MODE=true, replacing live Prometheus queries.
type MockDataProvider struct {
	cfg        models.Config
	rng        *rand.Rand
	namespaces []string
	workloads  []models.RawWorkload
}

var containerNames = []string{
	"nginx", "api-server", "redis", "postgres", "worker",
	"sidecar-proxy", "metrics-exporter", "envoy", "fluentd",
	"memcached", "rabbitmq", "celery-worker", "grpc-server",
	"graphql-gateway", "auth-service", "config-reloader",
	"vault-agent", "istio-proxy", "otel-collector", "log-router",
}

var imageRegistry = map[string]string{
	"nginx":           "nginx:1.25",
	"api-server":      "company/api-server:3.12.1",
	"redis":           "redis:7.2",
	"postgres":        "postgres:16",
	"worker":          "company/worker:2.8.0",
	"sidecar-proxy":   "envoyproxy/envoy:v1.28",
	"metrics-exporter": "prom/node-exporter:1.7",
	"envoy":           "envoyproxy/envoy:v1.28",
	"fluentd":         "fluent/fluentd:v1.16",
	"memcached":       "memcached:1.6",
	"rabbitmq":        "rabbitmq:3.12-management",
	"celery-worker":   "company/celery-worker:1.4.2",
	"grpc-server":     "company/grpc-server:2.1.0",
	"graphql-gateway": "company/graphql-gw:1.9.3",
	"auth-service":    "company/auth:4.0.1",
	"config-reloader": "jimmidyson/configmap-reload:v0.9.0",
	"vault-agent":     "hashicorp/vault:1.15",
	"istio-proxy":     "istio/proxyv2:1.20",
	"otel-collector":  "otel/opentelemetry-collector:0.91",
	"log-router":      "fluent/fluent-bit:2.2",
}

type workloadTemplate struct {
	name       string
	namespace  string
	wType      models.WorkloadType
	replicas   int
	containers []containerTemplate
}

type containerTemplate struct {
	nameIdx     int
	cpuReq      float64
	memReqGiB   float64
	cpuLim      float64
	memLimGiB   float64
	usageFactor float64 // fraction of request actually used (>1 means under-provisioned)
	noLimits    bool
}

func NewMockDataProvider(cfg models.Config) *MockDataProvider {
	m := &MockDataProvider{
		cfg: cfg,
		rng: rand.New(rand.NewSource(42)),
		namespaces: []string{
			"production", "staging", "monitoring",
			"kube-system", "data-pipeline", "frontend", "backend",
		},
	}
	m.workloads = m.generateWorkloads()
	return m
}

func (m *MockDataProvider) GetNamespaces() []string {
	out := make([]string, len(m.namespaces))
	copy(out, m.namespaces)
	return out
}

func (m *MockDataProvider) GetWorkloads(namespace string) []models.RawWorkload {
	var out []models.RawWorkload
	for _, w := range m.workloads {
		if w.Namespace == namespace {
			out = append(out, w)
		}
	}
	return out
}

func (m *MockDataProvider) GetAllWorkloads() []models.RawWorkload {
	out := make([]models.RawWorkload, len(m.workloads))
	copy(out, m.workloads)
	return out
}

func (m *MockDataProvider) generateWorkloads() []models.RawWorkload {
	templates := []workloadTemplate{
		// --- production (high-replica, well-provisioned + a few wasteful) ---
		{name: "web-frontend", namespace: "production", wType: models.Deployment, replicas: 6, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.35},
			{nameIdx: 17, cpuReq: 0.1, memReqGiB: 0.128, cpuLim: 0.2, memLimGiB: 0.256, usageFactor: 0.50},
		}},
		{name: "api-gateway", namespace: "production", wType: models.Deployment, replicas: 4, containers: []containerTemplate{
			{nameIdx: 1, cpuReq: 2.0, memReqGiB: 4.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.25},
			{nameIdx: 5, cpuReq: 0.1, memReqGiB: 0.064, cpuLim: 0.2, memLimGiB: 0.128, usageFactor: 0.40},
		}},
		{name: "order-service", namespace: "production", wType: models.Deployment, replicas: 3, containers: []containerTemplate{
			{nameIdx: 12, cpuReq: 1.5, memReqGiB: 3.0, cpuLim: 3.0, memLimGiB: 6.0, usageFactor: 0.60},
			{nameIdx: 6, cpuReq: 0.05, memReqGiB: 0.032, cpuLim: 0.1, memLimGiB: 0.064, usageFactor: 0.30},
		}},
		{name: "payment-processor", namespace: "production", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 14, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.70},
		}},
		{name: "postgres-primary", namespace: "production", wType: models.StatefulSet, replicas: 1, containers: []containerTemplate{
			{nameIdx: 3, cpuReq: 4.0, memReqGiB: 8.0, cpuLim: 8.0, memLimGiB: 16.0, usageFactor: 0.45},
			{nameIdx: 6, cpuReq: 0.05, memReqGiB: 0.032, cpuLim: 0.1, memLimGiB: 0.064, usageFactor: 0.20},
		}},
		{name: "redis-cache", namespace: "production", wType: models.StatefulSet, replicas: 3, containers: []containerTemplate{
			{nameIdx: 2, cpuReq: 1.0, memReqGiB: 4.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.80},
		}},
		{name: "notification-svc", namespace: "production", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 0.5, memReqGiB: 0.512, cpuLim: 1.0, memLimGiB: 1.0, usageFactor: 0.15},
			{nameIdx: 17, cpuReq: 0.1, memReqGiB: 0.128, cpuLim: 0.2, memLimGiB: 0.256, usageFactor: 0.45},
		}},

		// --- staging (mirrors production but smaller, some over-provisioned) ---
		{name: "web-frontend", namespace: "staging", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.10},
			{nameIdx: 17, cpuReq: 0.1, memReqGiB: 0.128, cpuLim: 0.2, memLimGiB: 0.256, usageFactor: 0.20},
		}},
		{name: "api-gateway", namespace: "staging", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 1, cpuReq: 2.0, memReqGiB: 4.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.08},
			{nameIdx: 5, cpuReq: 0.1, memReqGiB: 0.064, cpuLim: 0.2, memLimGiB: 0.128, usageFactor: 0.15},
		}},
		{name: "order-service", namespace: "staging", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 12, cpuReq: 1.5, memReqGiB: 3.0, cpuLim: 3.0, memLimGiB: 6.0, usageFactor: 0.05},
		}},
		{name: "postgres-staging", namespace: "staging", wType: models.StatefulSet, replicas: 1, containers: []containerTemplate{
			{nameIdx: 3, cpuReq: 2.0, memReqGiB: 4.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.12},
		}},
		{name: "redis-staging", namespace: "staging", wType: models.StatefulSet, replicas: 1, containers: []containerTemplate{
			{nameIdx: 2, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.10},
		}},

		// --- monitoring (observability stack) ---
		{name: "prometheus", namespace: "monitoring", wType: models.StatefulSet, replicas: 2, containers: []containerTemplate{
			{nameIdx: 6, cpuReq: 2.0, memReqGiB: 8.0, cpuLim: 4.0, memLimGiB: 12.0, usageFactor: 0.65},
			{nameIdx: 15, cpuReq: 0.01, memReqGiB: 0.01, cpuLim: 0.05, memLimGiB: 0.05, usageFactor: 0.50},
		}},
		{name: "grafana", namespace: "monitoring", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.30},
		}},
		{name: "alertmanager", namespace: "monitoring", wType: models.StatefulSet, replicas: 2, containers: []containerTemplate{
			{nameIdx: 6, cpuReq: 0.1, memReqGiB: 0.256, cpuLim: 0.2, memLimGiB: 0.512, usageFactor: 0.25},
		}},
		{name: "otel-collector", namespace: "monitoring", wType: models.Deployment, replicas: 3, containers: []containerTemplate{
			{nameIdx: 18, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.55},
		}},
		{name: "fluentd-aggregator", namespace: "monitoring", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 8, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.70},
		}},

		// --- kube-system (infra, some with no limits = BestEffort) ---
		{name: "coredns", namespace: "kube-system", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 0.1, memReqGiB: 0.070, cpuLim: 0.0, memLimGiB: 0.170, usageFactor: 0.55},
		}},
		{name: "kube-proxy", namespace: "kube-system", wType: models.Deployment, replicas: 3, containers: []containerTemplate{
			{nameIdx: 5, cpuReq: 0.0, memReqGiB: 0.0, cpuLim: 0.0, memLimGiB: 0.0, usageFactor: 0.0, noLimits: true},
		}},
		{name: "metrics-server", namespace: "kube-system", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 6, cpuReq: 0.1, memReqGiB: 0.200, cpuLim: 0.1, memLimGiB: 0.200, usageFactor: 0.90},
		}},
		{name: "cluster-autoscaler", namespace: "kube-system", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 1, cpuReq: 0.1, memReqGiB: 0.300, cpuLim: 0.5, memLimGiB: 0.600, usageFactor: 0.20},
		}},
		{name: "vault-agent-injector", namespace: "kube-system", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 16, cpuReq: 0.25, memReqGiB: 0.256, cpuLim: 0.25, memLimGiB: 0.256, usageFactor: 0.40},
		}},

		// --- data-pipeline (heavy workloads, some under-provisioned / OOMKill risk) ---
		{name: "spark-driver", namespace: "data-pipeline", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 2.0, memReqGiB: 4.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 1.15},
		}},
		{name: "spark-executor", namespace: "data-pipeline", wType: models.Deployment, replicas: 8, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 4.0, memReqGiB: 8.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.92},
		}},
		{name: "kafka-broker", namespace: "data-pipeline", wType: models.StatefulSet, replicas: 3, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 2.0, memReqGiB: 6.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.75},
			{nameIdx: 6, cpuReq: 0.05, memReqGiB: 0.064, cpuLim: 0.1, memLimGiB: 0.128, usageFactor: 0.40},
		}},
		{name: "flink-jobmanager", namespace: "data-pipeline", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 1.25},
		}},
		{name: "flink-taskmanager", namespace: "data-pipeline", wType: models.StatefulSet, replicas: 4, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 2.0, memReqGiB: 4.0, cpuLim: 4.0, memLimGiB: 8.0, usageFactor: 0.85},
		}},
		{name: "rabbitmq", namespace: "data-pipeline", wType: models.StatefulSet, replicas: 3, containers: []containerTemplate{
			{nameIdx: 10, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.60},
		}},
		{name: "etl-cron-job", namespace: "data-pipeline", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 0.0, memLimGiB: 0.0, usageFactor: 0.0, noLimits: true},
		}},

		// --- frontend (UI services, mostly over-provisioned) ---
		{name: "react-app", namespace: "frontend", wType: models.Deployment, replicas: 3, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 0.5, memReqGiB: 0.512, cpuLim: 1.0, memLimGiB: 1.0, usageFactor: 0.12},
		}},
		{name: "next-ssr", namespace: "frontend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.20},
			{nameIdx: 17, cpuReq: 0.1, memReqGiB: 0.128, cpuLim: 0.2, memLimGiB: 0.256, usageFactor: 0.35},
		}},
		{name: "cdn-origin", namespace: "frontend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 0, cpuReq: 0.25, memReqGiB: 0.256, cpuLim: 0.5, memLimGiB: 0.512, usageFactor: 0.18},
		}},
		{name: "image-resizer", namespace: "frontend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 2.0, memReqGiB: 2.0, cpuLim: 4.0, memLimGiB: 4.0, usageFactor: 0.30},
		}},

		// --- backend (core services, mix of waste and risk) ---
		{name: "user-service", namespace: "backend", wType: models.Deployment, replicas: 3, containers: []containerTemplate{
			{nameIdx: 12, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.45},
			{nameIdx: 5, cpuReq: 0.1, memReqGiB: 0.064, cpuLim: 0.2, memLimGiB: 0.128, usageFactor: 0.40},
		}},
		{name: "inventory-service", namespace: "backend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 12, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.55},
		}},
		{name: "search-engine", namespace: "backend", wType: models.StatefulSet, replicas: 3, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 4.0, memReqGiB: 8.0, cpuLim: 8.0, memLimGiB: 16.0, usageFactor: 0.40},
			{nameIdx: 6, cpuReq: 0.05, memReqGiB: 0.032, cpuLim: 0.1, memLimGiB: 0.064, usageFactor: 0.25},
		}},
		{name: "email-worker", namespace: "backend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 11, cpuReq: 0.25, memReqGiB: 0.512, cpuLim: 0.5, memLimGiB: 1.0, usageFactor: 1.10},
		}},
		{name: "graphql-gateway", namespace: "backend", wType: models.Deployment, replicas: 2, containers: []containerTemplate{
			{nameIdx: 13, cpuReq: 1.0, memReqGiB: 2.0, cpuLim: 2.0, memLimGiB: 4.0, usageFactor: 0.50},
			{nameIdx: 17, cpuReq: 0.1, memReqGiB: 0.128, cpuLim: 0.2, memLimGiB: 0.256, usageFactor: 0.45},
		}},
		{name: "cache-warmer", namespace: "backend", wType: models.Deployment, replicas: 1, containers: []containerTemplate{
			{nameIdx: 4, cpuReq: 0.5, memReqGiB: 0.5, cpuLim: 0.0, memLimGiB: 0.0, usageFactor: 0.0, noLimits: true},
			{nameIdx: 9, cpuReq: 0.5, memReqGiB: 1.0, cpuLim: 1.0, memLimGiB: 2.0, usageFactor: 0.35},
		}},
	}

	workloads := make([]models.RawWorkload, 0, len(templates))
	for _, t := range templates {
		workloads = append(workloads, m.buildWorkload(t))
	}
	return workloads
}

func (m *MockDataProvider) buildWorkload(t workloadTemplate) models.RawWorkload {
	containers := make([]models.RawContainer, 0, len(t.containers))
	for _, ct := range t.containers {
		containers = append(containers, m.buildContainer(ct))
	}
	return models.RawWorkload{
		Name:       t.name,
		Namespace:  t.namespace,
		Type:       t.wType,
		Replicas:   t.replicas,
		Containers: containers,
	}
}

func (m *MockDataProvider) buildContainer(ct containerTemplate) models.RawContainer {
	name := containerNames[ct.nameIdx]
	image := imageRegistry[name]

	if ct.noLimits {
		return models.RawContainer{
			Name:           name,
			Image:          image,
			CurrentRequest: models.ResourceValues{CPUCores: ct.cpuReq, MemoryGiB: ct.memReqGiB},
			CurrentLimit:   models.ResourceValues{CPUCores: 0, MemoryGiB: 0},
			Usage:          m.generateBestEffortUsage(),
		}
	}

	baseUsageCPU := ct.cpuReq * ct.usageFactor
	baseUsageMem := ct.memReqGiB * ct.usageFactor

	return models.RawContainer{
		Name:           name,
		Image:          image,
		CurrentRequest: models.ResourceValues{CPUCores: ct.cpuReq, MemoryGiB: ct.memReqGiB},
		CurrentLimit:   models.ResourceValues{CPUCores: ct.cpuLim, MemoryGiB: ct.memLimGiB},
		Usage:          m.generateUsageStats(baseUsageCPU, baseUsageMem),
	}
}

func (m *MockDataProvider) generateUsageStats(baseCPU, baseMem float64) models.UsageStats {
	return models.UsageStats{
		Average: models.ResourceValues{CPUCores: m.jitter(baseCPU, 0.05), MemoryGiB: m.jitter(baseMem, 0.05)},
		P50:     models.ResourceValues{CPUCores: m.jitter(baseCPU*0.95, 0.05), MemoryGiB: m.jitter(baseMem*0.95, 0.05)},
		P90:     models.ResourceValues{CPUCores: m.jitter(baseCPU*1.20, 0.05), MemoryGiB: m.jitter(baseMem*1.15, 0.05)},
		P95:     models.ResourceValues{CPUCores: m.jitter(baseCPU*1.35, 0.05), MemoryGiB: m.jitter(baseMem*1.25, 0.05)},
		P99:     models.ResourceValues{CPUCores: m.jitter(baseCPU*1.55, 0.05), MemoryGiB: m.jitter(baseMem*1.40, 0.05)},
		Max:     models.ResourceValues{CPUCores: m.jitter(baseCPU*1.80, 0.08), MemoryGiB: m.jitter(baseMem*1.60, 0.08)},
	}
}

func (m *MockDataProvider) generateBestEffortUsage() models.UsageStats {
	cpuBase := 0.05 + m.rng.Float64()*0.15
	memBase := 0.02 + m.rng.Float64()*0.10
	return m.generateUsageStats(cpuBase, memBase)
}

// jitter returns v adjusted by a random factor in [-pct, +pct], clamped to >= 0.001.
func (m *MockDataProvider) jitter(v, pct float64) float64 {
	delta := v * pct * (2*m.rng.Float64() - 1)
	result := v + delta
	if result < 0.001 {
		return 0.001
	}
	return result
}
