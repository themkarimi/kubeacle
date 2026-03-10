# Kubeacle — Kubernetes Workload Rightsizing Service

![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

## Overview

Kubeacle connects to a Prometheus instance, analyzes CPU and memory requests and limits on Kubernetes workloads (Deployments and StatefulSets), evaluates real usage patterns over a configurable lookback window, and generates intelligent rightsizing recommendations. It surfaces over-provisioned and under-provisioned containers, estimates cost savings, and exports corrective patches as YAML or Helm values — giving platform teams a clear, actionable path to better resource efficiency.

## Architecture

```
┌────────────┐      ┌────────────────┐      ┌────────────┐
│  React UI  │─────▶│  Go Backend    │─────▶│ Prometheus  │
│  (Vite)    │ :3000│  (Chi router)  │ :8080│             │ :9090
└────────────┘      └────────────────┘      └────────────┘
```

| Layer | Technology | Details |
|-------|-----------|---------|
| **Backend** | Go 1.23, Chi v5 | REST API, analysis engine, in-memory cache, mock-mode support |
| **Frontend** | React 18, Recharts, Vite | Interactive dashboard with charts, filtering, and export controls |
| **Metrics** | Prometheus | Collects and serves container resource usage time-series |
| **Orchestration** | Docker Compose | Single-command local development stack |

## Project Structure

```
kubeacle/
├── docker-compose.yml
├── prometheus.yml
├── README.md
├── rightsizer/                        # Go backend
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── server/
│   │       └── main.go               # Entry point & config loading
│   └── internal/
│       ├── analyzer/
│       │   ├── engine.go             # Rightsizing analysis engine
│       │   └── engine_test.go
│       ├── api/
│       │   ├── router.go             # Route definitions (Chi)
│       │   ├── handlers.go           # HTTP handlers
│       │   └── handlers_test.go
│       ├── mock/
│       │   ├── data.go               # Deterministic fake workloads
│       │   └── data_test.go
│       ├── models/
│       │   └── types.go              # Shared data structures
│       └── prometheus/
│           └── client.go             # Prometheus query client
└── ui/                                # React frontend
    ├── Dockerfile
    ├── index.html
    ├── nginx.conf                     # Production reverse-proxy config
    ├── package.json
    ├── vite.config.js
    ├── public/
    └── src/
        ├── main.jsx                   # React entry point
        └── App.jsx                    # Main application component
```

## Quick Start

```bash
# Clone the repository
git clone https://github.com/themkarimi/kubeacle.git
cd kubeacle

# Start all services (mock mode by default)
docker-compose up --build

# Access:
# - UI:         http://localhost:3000
# - API:        http://localhost:8080
# - Prometheus: http://localhost:9090
```

## Local Development

### Backend

```bash
cd rightsizer
go run ./cmd/server
# Server starts on :8080 with MOCK_MODE=true
```

### Frontend

```bash
cd ui
npm install
npm run dev
# Dev server starts on :3000 with API proxy to :8080
```

The Vite dev server proxies `/api` and `/health` requests to the backend automatically.

## API Endpoints

All data endpoints live under the `/api/v1` prefix and return JSON.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check — returns `{"status":"ok","mode":"mock\|live"}` |
| `GET` | `/api/v1/namespaces` | List all discovered namespaces |
| `GET` | `/api/v1/workloads` | List workloads (query params: `namespace`, `type`, `page`, `page_size`) |
| `GET` | `/api/v1/workloads/{namespace}/{name}/analysis` | Detailed analysis for a single workload |
| `GET` | `/api/v1/recommendations` | Cluster-wide recommendations (filters: `namespace`, `risk`, `sort`) |
| `GET` | `/api/v1/cluster/summary` | Cluster summary — totals, waste %, risk distribution, namespace breakdowns |
| `POST` | `/api/v1/export/yaml` | Export recommendations as YAML patches (`{"workload_ids":["ns/name"]}`) |
| `POST` | `/api/v1/export/helm` | Export recommendations as Helm values JSON |
| `GET` | `/api/v1/config` | Retrieve current runtime configuration |
| `PUT` | `/api/v1/config` | Update configuration (headroom, percentile, costs, exclusions) |
| `GET` | `/api/v1/prometheus/health` | Check Prometheus connectivity |

## Configuration

All settings are controlled via environment variables with sensible defaults.

| Variable | Default | Description |
|----------|---------|-------------|
| `MOCK_MODE` | `true` | Use generated fake data instead of live Prometheus queries |
| `PORT` | `8080` | HTTP server listen port |
| `PROMETHEUS_URL` | `http://localhost:9090` | Prometheus server URL |
| `LOOKBACK_WINDOW` | `168h` | Historical analysis window (Go duration) |
| `HEADROOM_FACTOR` | `0.20` | Safety margin added on top of recommendations (20 %) |
| `SPIKE_PERCENTILE` | `P95` | Percentile used for limit calculations (`P90`, `P95`, `P99`, `Max`) |
| `COST_PER_CPU_HOUR` | `0.031611` | USD per CPU-core per hour for savings estimates |
| `COST_PER_GIB_HOUR` | `0.004237` | USD per GiB per hour for savings estimates |
| `EXCLUDE_NAMESPACES` | `kube-system` | Comma-separated namespaces to skip |
| `REFRESH_INTERVAL` | `5m` | Cache TTL for analysis results |

## Mock Mode

When `MOCK_MODE=true` (the default), the backend generates realistic fake data without requiring a live Kubernetes cluster or Prometheus instance. The mock provider is seeded deterministically and creates:

- **7 namespaces** — `production`, `staging`, `monitoring`, `kube-system`, `data-pipeline`, `frontend`, `backend`
- **~40 workloads** — a mix of Deployments and StatefulSets with realistic replica counts
- **Varied usage patterns** — well-provisioned (30–70 % utilisation), over-provisioned (5–20 %), under-provisioned (85–125 %), and BestEffort (no limits)
- **Full metrics** — average, P50, P90, P95, P99, and max usage with ±5 % jitter

This makes it easy to demo, develop, and test without any cluster infrastructure.

## Kubernetes Deployment

To run against a real cluster:

1. Set `MOCK_MODE=false`.
2. Point `PROMETHEUS_URL` to your in-cluster Prometheus instance (e.g. `http://prometheus-server.monitoring.svc:9090`).
3. Ensure Prometheus is scraping `container_cpu_usage_seconds_total` and `container_memory_working_set_bytes` metrics from your kubelets / cAdvisor.

```bash
docker-compose up --build -e MOCK_MODE=false -e PROMETHEUS_URL=http://prometheus:9090
```

## Testing

```bash
cd rightsizer
go test ./... -v
```

Tests cover the analysis engine, API handlers, and mock data provider.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language (backend) | Go 1.23 |
| Router | [Chi v5](https://github.com/go-chi/chi) |
| CORS | [rs/cors](https://github.com/rs/cors) |
| Language (frontend) | JavaScript (React 18) |
| Charts | [Recharts](https://recharts.org/) |
| Build tool | [Vite](https://vitejs.dev/) |
| Production server | Nginx |
| Metrics | Prometheus |
| Containerisation | Docker & Docker Compose |

## License

This project is licensed under the [MIT License](LICENSE).