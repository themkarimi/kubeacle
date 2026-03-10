package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/themkarimi/kubeacle/rightsizer/internal/analyzer"
	"github.com/themkarimi/kubeacle/rightsizer/internal/mock"
	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
	"github.com/themkarimi/kubeacle/rightsizer/internal/prometheus"
)

type Server struct {
	cfg        models.Config
	engine     *analyzer.Engine
	promClient *prometheus.Client
	mockData   *mock.MockDataProvider
	router     chi.Router
	cache      *AnalysisCache
}

func NewServer(cfg models.Config) *Server {
	s := &Server{
		cfg:    cfg,
		engine: analyzer.NewEngine(cfg),
	}

	if cfg.MockMode {
		s.mockData = mock.NewMockDataProvider(cfg)
	} else {
		s.promClient = prometheus.NewClient(cfg.PrometheusURL, cfg.LookbackWindow)
	}

	s.cache = NewAnalysisCache(cfg.RefreshInterval)
	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.RequestID)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	})
	r.Use(c.Handler)

	r.Get("/health", s.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/namespaces", s.handleGetNamespaces)
		r.Get("/workloads", s.handleGetWorkloads)
		r.Get("/workloads/{namespace}/{name}/analysis", s.handleGetWorkloadAnalysis)
		r.Get("/recommendations", s.handleGetRecommendations)
		r.Get("/cluster/summary", s.handleGetClusterSummary)
		r.Get("/config", s.handleGetConfig)
		r.Put("/config", s.handleUpdateConfig)
		r.Post("/config/test-prometheus", s.handleTestPrometheus)
		r.Get("/prometheus/health", s.handlePrometheusHealth)
	})

	s.router = r
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
