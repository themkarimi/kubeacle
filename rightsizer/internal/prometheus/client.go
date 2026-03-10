package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/themkarimi/kubeacle/rightsizer/internal/models"
)

// Client wraps the Prometheus HTTP API
type Client struct {
	baseURL    string
	httpClient *http.Client
	lookback   time.Duration
}

// NewClient creates a new Prometheus client
func NewClient(baseURL string, lookback time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		lookback:   lookback,
	}
}

// HealthCheck tests connectivity to Prometheus
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/-/healthy", nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("prometheus health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}
	return nil
}

// GetNamespaces queries Prometheus for distinct namespace labels.
// In a real implementation, this would query:
// group by (namespace) (kube_pod_container_resource_requests)
func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented: use mock mode")
}

// GetWorkloads queries Prometheus for workload resource data in a namespace.
// In a real implementation, this would query:
// kube_pod_container_resource_requests{resource="cpu", namespace=~"$ns"}
// kube_pod_container_resource_requests{resource="memory", namespace=~"$ns"}
func (c *Client) GetWorkloads(ctx context.Context, namespace string) ([]models.RawWorkload, error) {
	return nil, fmt.Errorf("not implemented: use mock mode")
}

// GetAllWorkloads queries for all workloads across all namespaces.
func (c *Client) GetAllWorkloads(ctx context.Context) ([]models.RawWorkload, error) {
	return nil, fmt.Errorf("not implemented: use mock mode")
}

// LookbackWindow returns the configured lookback duration string for PromQL.
func (c *Client) LookbackWindow() string {
	hours := int(c.lookback.Hours())
	if hours >= 24 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dh", hours)
}
