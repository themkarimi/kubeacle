# Kubeacle Helm Chart

A Helm chart for deploying [Kubeacle](https://github.com/themkarimi/kubeacle) – a Kubernetes workload rightsizing and cost-optimisation dashboard.

## Components

| Component | Description | Default port |
|-----------|-------------|-------------|
| **backend** (rightsizer) | Go REST API – analyses CPU/memory usage and generates rightsizing recommendations | 8080 |
| **frontend** (ui) | React + Nginx dashboard | 80 |
| **prometheus** *(optional)* | Bundled Prometheus for metrics storage | 9090 |

## Prerequisites

- Kubernetes 1.23+
- Helm 3.9+

## Installing the Chart

```bash
helm install my-kubeacle ./helm/kubeacle
```

Install into a dedicated namespace:

```bash
helm install my-kubeacle ./helm/kubeacle --namespace kubeacle --create-namespace
```

## Uninstalling the Chart

```bash
helm uninstall my-kubeacle
```

## Configuration

The following table lists key configurable parameters and their defaults. Override them with `--set key=value` or a custom `values.yaml` file.

### Global

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Partial override of the release name | `""` |
| `fullnameOverride` | Full override of the release name | `""` |
| `serviceAccount.create` | Create a dedicated ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name (auto-generated if empty) | `""` |

### Backend

| Parameter | Description | Default |
|-----------|-------------|---------|
| `backend.enabled` | Enable the backend component | `true` |
| `backend.image.repository` | Backend image repository | `ghcr.io/themkarimi/kubeacle/rightsizer` |
| `backend.image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `backend.replicaCount` | Number of backend replicas | `1` |
| `backend.service.type` | Service type | `ClusterIP` |
| `backend.service.port` | Service port | `8080` |
| `backend.config.mockMode` | Use deterministic mock data (`true`) or live Prometheus (`false`) | `"true"` |
| `backend.config.prometheusUrl` | Prometheus API endpoint | `"http://kubeacle-prometheus:9090"` |
| `backend.config.lookbackWindow` | Historical analysis window (Go duration) | `"168h"` |
| `backend.config.headroomFactor` | Safety headroom on recommendations | `"0.20"` |
| `backend.config.spikePercentile` | Spike baseline percentile (`P90`/`P95`/`P99`/`Max`) | `"P95"` |
| `backend.config.costPerCpuHour` | USD per CPU-core per hour | `"0.031611"` |
| `backend.config.costPerGibHour` | USD per GiB memory per hour | `"0.004237"` |
| `backend.config.excludeNamespaces` | Comma-separated namespaces to skip | `"kube-system"` |
| `backend.config.refreshInterval` | Analysis cache TTL (Go duration) | `"5m"` |

### Frontend

| Parameter | Description | Default |
|-----------|-------------|---------|
| `frontend.enabled` | Enable the frontend component | `true` |
| `frontend.image.repository` | Frontend image repository | `ghcr.io/themkarimi/kubeacle/ui` |
| `frontend.image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `frontend.replicaCount` | Number of frontend replicas | `1` |
| `frontend.service.type` | Service type | `ClusterIP` |
| `frontend.service.port` | Service port | `80` |

### Prometheus

| Parameter | Description | Default |
|-----------|-------------|---------|
| `prometheus.enabled` | Deploy a bundled Prometheus instance | `true` |
| `prometheus.image.tag` | Prometheus image tag | `"v3.2.1"` |
| `prometheus.retention` | TSDB retention period | `"15d"` |
| `prometheus.scrapeInterval` | How often to scrape the backend | `"30s"` |
| `prometheus.persistence.enabled` | Enable persistent storage | `false` |
| `prometheus.persistence.size` | PVC size | `10Gi` |
| `prometheus.persistence.storageClass` | StorageClass name | `""` (cluster default) |

### Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Create an Ingress resource | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress host rules | see `values.yaml` |
| `ingress.tls` | TLS configuration | `[]` |

## Usage Examples

### Enable live Prometheus mode

```bash
helm install my-kubeacle ./helm/kubeacle \
  --set backend.config.mockMode=false \
  --set backend.config.prometheusUrl=http://my-prometheus:9090
```

### Expose the dashboard via an Ingress

```bash
helm install my-kubeacle ./helm/kubeacle \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set "ingress.hosts[0].host=kubeacle.example.com" \
  --set "ingress.hosts[0].paths[0].path=/" \
  --set "ingress.hosts[0].paths[0].pathType=Prefix"
```

### Expose the dashboard via NodePort (quick access)

```bash
helm install my-kubeacle ./helm/kubeacle \
  --set frontend.service.type=NodePort
```

### Enable Prometheus persistence

```bash
helm install my-kubeacle ./helm/kubeacle \
  --set prometheus.persistence.enabled=true \
  --set prometheus.persistence.size=20Gi
```

### Disable the bundled Prometheus (external instance)

```bash
helm install my-kubeacle ./helm/kubeacle \
  --set prometheus.enabled=false \
  --set backend.config.mockMode=false \
  --set backend.config.prometheusUrl=http://existing-prometheus.monitoring:9090
```

## Port-forwarding (ClusterIP default)

```bash
# Dashboard
kubectl port-forward svc/my-kubeacle-kubeacle-frontend 8080:80

# Backend API
kubectl port-forward svc/my-kubeacle-kubeacle-backend 8081:8080

# Prometheus
kubectl port-forward svc/my-kubeacle-kubeacle-prometheus 9090:9090
```
