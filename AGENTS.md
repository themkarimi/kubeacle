# Agents Guide — Kubeacle

This file provides guidance for AI coding agents working in this repository.

## Project Overview

Kubeacle is a full-stack Kubernetes workload rightsizing service. It connects to a Prometheus-compatible metrics backend, analyzes CPU and memory usage on Deployments and StatefulSets, and generates rightsizing recommendations with estimated cost savings and YAML/Helm export support.

## Repository Layout

```
kubeacle/
├── docker-compose.yml          # Local development stack (mock mode by default)
├── prometheus.yml              # Prometheus scrape configuration
├── README.md
├── AGENTS.md                   # This file
├── helm/                       # Helm chart for Kubernetes deployment
├── rightsizer/                 # Go backend (service root)
│   ├── Dockerfile
│   ├── go.mod / go.sum
│   ├── cmd/server/main.go      # Entry point & environment-variable config
│   └── internal/
│       ├── analyzer/engine.go  # Rightsizing analysis logic
│       ├── api/
│       │   ├── router.go       # Chi route definitions
│       │   └── handlers.go     # HTTP handlers
│       ├── mock/data.go        # Deterministic fake workloads (mock mode)
│       ├── models/types.go     # Shared Go data structures (JSON tags: snake_case)
│       └── prometheus/client.go# Prometheus / VictoriaMetrics query client
└── ui/                         # React frontend
    ├── Dockerfile
    ├── nginx.conf              # Production reverse-proxy config
    ├── vite.config.js          # Vite build + dev-proxy config
    └── src/
        ├── main.jsx            # React entry point
        └── App.jsx             # Entire UI — all views and components live here
```

## Building

### Backend (Go)

```bash
cd rightsizer
go build ./cmd/server
```

### Frontend (React / Vite)

```bash
cd ui
npm install
npm run build
```

### Full stack (Docker Compose)

```bash
docker-compose up --build
# UI:  http://localhost:3000
# API: http://localhost:8080
```

## Running Tests

### Backend

```bash
cd rightsizer
go test ./...
```

Tests cover the analysis engine (`analyzer/`), HTTP handlers (`api/`), and mock data provider (`mock/`).

There are no frontend tests at this time.

## Local Development

```bash
# Backend (mock mode on by default)
cd rightsizer && go run ./cmd/server   # listens on :8080

# Frontend dev server (proxies /api and /health to :8080)
cd ui && npm install && npm run dev    # listens on :3000
```

## Code Conventions

### Go

- All Go code lives under `rightsizer/internal/`.
- JSON response field names use **snake_case** (e.g. `overall_waste_score`, `estimated_monthly_saving_usd`, `mem_waste_percent`). Mirror this in any new struct tags in `models/types.go`.
- The router uses [Chi v5](https://github.com/go-chi/chi). Add new routes in `api/router.go` and their handlers in `api/handlers.go`.
- Environment variables (see `cmd/server/main.go`) control all runtime configuration; do not hardcode values.

### JavaScript / React

- The entire frontend lives in a **single file**: `ui/src/App.jsx`. All new views and components belong there.
- Use the existing colour constants (`C.*`) and font variable (`FONT.mono`) rather than raw CSS literals.

## Key Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `MOCK_MODE` | `true` | Use fake data; set `false` for a real cluster |
| `PORT` | `8080` | Backend HTTP port |
| `PROMETHEUS_URL` | `http://localhost:9090` | Prometheus or VictoriaMetrics base URL |
| `LOOKBACK_WINDOW` | `168h` | Historical window for analysis |
| `HEADROOM_FACTOR` | `0.20` | Safety margin added to recommendations |
| `SPIKE_PERCENTILE` | `P95` | Percentile used for limit calculations |

## API Overview

All data endpoints are under `/api/v1` and return JSON.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/namespaces` | List namespaces |
| `GET` | `/api/v1/workloads` | List workloads |
| `GET` | `/api/v1/workloads/{namespace}/{name}/analysis` | Single workload analysis |
| `GET` | `/api/v1/recommendations` | Cluster-wide recommendations |
| `GET` | `/api/v1/cluster/summary` | Cluster summary |
| `POST` | `/api/v1/export/yaml` | Export YAML patches |
| `POST` | `/api/v1/export/helm` | Export Helm values |
| `GET`/`PUT` | `/api/v1/config` | Read / update runtime config |
| `GET` | `/api/v1/prometheus/health` | Prometheus connectivity check |

## Notes for Agents

- **Mock mode is on by default.** You do not need a live Kubernetes cluster or Prometheus to run or test the service.
- When adding a new API field, update `models/types.go` first, then the analysis logic, then the handler, and finally the frontend rendering in `App.jsx`.
- Keep the `go.mod` module path (`github.com/themkarimi/kubeacle/rightsizer`) intact when adding new packages.
- The Vite dev proxy (`vite.config.js`) forwards `/api` and `/health` to `http://localhost:8080`; there is no need to change CORS settings during local development.
- After modifying Go code, run `go test ./...` from `rightsizer/` to confirm nothing is broken before committing.
