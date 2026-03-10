package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/api"
	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

func main() {
	cfg := loadConfig()

	log.Printf("Starting Kubernetes Rightsizing Service on port %d", cfg.Port)
	log.Printf("Mock mode: %v", cfg.MockMode)

	server := api.NewServer(cfg)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	log.Printf("Server listening on :%d", cfg.Port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped")
}

func loadConfig() models.Config {
	cfg := models.Config{
		PrometheusURL:     getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		LookbackWindow:    parseDuration(getEnv("LOOKBACK_WINDOW", "168h")),
		HeadroomFactor:    parseFloat(getEnv("HEADROOM_FACTOR", "0.20")),
		SpikePercentile:   getEnv("SPIKE_PERCENTILE", "P95"),
		CostPerCPUHour:    parseFloat(getEnv("COST_PER_CPU_HOUR", "0.031611")),
		CostPerGiBHour:    parseFloat(getEnv("COST_PER_GIB_HOUR", "0.004237")),
		ExcludeNamespaces: parseStringSlice(getEnv("EXCLUDE_NAMESPACES", "kube-system")),
		MockMode:          parseBool(getEnv("MOCK_MODE", "true")),
		Port:              parseInt(getEnv("PORT", "8080")),
		RefreshInterval:   parseDuration(getEnv("REFRESH_INTERVAL", "5m")),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Invalid duration %q, defaulting to 5m: %v", s, err)
		return 5 * time.Minute
	}
	return d
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("Invalid float %q, defaulting to 0: %v", s, err)
		return 0
	}
	return f
}

func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		log.Printf("Invalid bool %q, defaulting to false: %v", s, err)
		return false
	}
	return b
}

func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("Invalid int %q, defaulting to 8080: %v", s, err)
		return 8080
	}
	return i
}

func parseStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
