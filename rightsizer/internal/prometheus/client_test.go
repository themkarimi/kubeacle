package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockPromServer creates a test server that responds to Prometheus API calls.
// handlers maps URL path to a handler function.
func mockPromServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	return httptest.NewServer(mux)
}

// promVectorResponse builds a minimal Prometheus instant-query JSON response.
// It panics on marshal error (only statically-typed test data is passed in).
func promVectorResponse(samples []map[string]interface{}) []byte {
	type sample struct {
		Metric map[string]string `json:"metric"`
		Value  []interface{}     `json:"value"`
	}
	var result []sample
	for _, s := range samples {
		metric := make(map[string]string)
		for k, v := range s {
			if str, ok := v.(string); ok {
				metric[k] = str
			}
		}
		val := "0"
		if v, ok := s["__value__"]; ok {
			val = v.(string)
		}
		result = append(result, sample{
			Metric: metric,
			Value:  []interface{}{1234567890.0, val},
		})
	}
	resp := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result":     result,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		panic("promVectorResponse: json.Marshal failed: " + err.Error())
	}
	return b
}

func TestHealthCheck(t *testing.T) {
	srv := mockPromServer(t, map[string]http.HandlerFunc{
		"/-/healthy": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("expected healthy, got: %v", err)
	}
}

func TestHealthCheckUnhealthy(t *testing.T) {
	srv := mockPromServer(t, map[string]http.HandlerFunc{
		"/-/healthy": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	if err := c.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error for unhealthy backend, got nil")
	}
}

func TestGetNamespaces(t *testing.T) {
	srv := mockPromServer(t, map[string]http.HandlerFunc{
		"/api/v1/label/namespace/values": func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"status": "success",
				"data":   []string{"default", "production", "staging"},
			}
			json.NewEncoder(w).Encode(resp)
		},
	})
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	namespaces, err := c.GetNamespaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(namespaces) != 3 {
		t.Fatalf("expected 3 namespaces, got %d", len(namespaces))
	}
}

func TestGetNamespacesError(t *testing.T) {
	srv := mockPromServer(t, map[string]http.HandlerFunc{
		"/api/v1/label/namespace/values": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		},
	})
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	_, err := c.GetNamespaces(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestGetAllWorkloads(t *testing.T) {
	// Serve a minimal but realistic set of Prometheus responses.
	// Two workloads: a Deployment "web" in "production" and
	// a StatefulSet "db" in "production".
	queryResponses := map[string][]byte{
		"kube_deployment_spec_replicas": promVectorResponse([]map[string]interface{}{
			{"namespace": "production", "deployment": "web", "__value__": "2"},
		}),
		"kube_statefulset_spec_replicas": promVectorResponse([]map[string]interface{}{
			{"namespace": "production", "statefulset": "db", "__value__": "1"},
		}),
		// pod_to_rs: pod "web-abc-xyz" owned by RS "web-abc"
		`kube_pod_owner{owner_kind="ReplicaSet"}`: promVectorResponse([]map[string]interface{}{
			{"namespace": "production", "pod": "web-abc-xyz", "owner_kind": "ReplicaSet", "owner_name": "web-abc", "__value__": "1"},
		}),
		// rs_to_dep: RS "web-abc" owned by Deployment "web"
		`kube_replicaset_owner{owner_kind="Deployment"}`: promVectorResponse([]map[string]interface{}{
			{"namespace": "production", "replicaset": "web-abc", "owner_kind": "Deployment", "owner_name": "web", "__value__": "1"},
		}),
		// pod_to_ss: pod "db-0" owned by StatefulSet "db"
		`kube_pod_owner{owner_kind="StatefulSet"}`: promVectorResponse([]map[string]interface{}{
			{"namespace": "production", "pod": "db-0", "owner_kind": "StatefulSet", "owner_name": "db", "__value__": "1"},
		}),
	}

	// Default empty response for all other queries (usage metrics).
	emptyResp, err := json.Marshal(map[string]interface{}{
		"status": "success",
		"data":   map[string]interface{}{"resultType": "vector", "result": []interface{}{}},
	})
	if err != nil {
		t.Fatalf("failed to build empty response: %v", err)
	}

	// Add resource requests/limits for both pods.
	queryResponses[`kube_pod_container_resource_requests{resource="cpu",container!=""}`] = promVectorResponse([]map[string]interface{}{
		{"namespace": "production", "pod": "web-abc-xyz", "container": "app", "__value__": "0.5"},
		{"namespace": "production", "pod": "db-0", "container": "db", "__value__": "1.0"},
	})
	queryResponses[`kube_pod_container_resource_requests{resource="memory",container!=""}`] = promVectorResponse([]map[string]interface{}{
		{"namespace": "production", "pod": "web-abc-xyz", "container": "app", "__value__": "536870912"}, // 0.5 GiB
		{"namespace": "production", "pod": "db-0", "container": "db", "__value__": "1073741824"},        // 1 GiB
	})
	queryResponses[`kube_pod_container_resource_limits{resource="cpu",container!=""}`] = promVectorResponse([]map[string]interface{}{
		{"namespace": "production", "pod": "web-abc-xyz", "container": "app", "__value__": "1.0"},
		{"namespace": "production", "pod": "db-0", "container": "db", "__value__": "2.0"},
	})
	queryResponses[`kube_pod_container_resource_limits{resource="memory",container!=""}`] = promVectorResponse([]map[string]interface{}{
		{"namespace": "production", "pod": "web-abc-xyz", "container": "app", "__value__": "1073741824"},
		{"namespace": "production", "pod": "db-0", "container": "db", "__value__": "2147483648"},
	})
	queryResponses[`kube_pod_container_info{container!=""}`] = promVectorResponse([]map[string]interface{}{
		{"namespace": "production", "pod": "web-abc-xyz", "container": "app", "image": "nginx:1.25", "__value__": "1"},
		{"namespace": "production", "pod": "db-0", "container": "db", "image": "postgres:15", "__value__": "1"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query" {
			query := r.URL.Query().Get("query")
			if resp, ok := queryResponses[query]; ok {
				w.Write(resp)
				return
			}
			w.Write(emptyResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	workloads, err := c.GetAllWorkloads(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d", len(workloads))
	}

	// workloads are sorted by namespace then name: "db" before "web"
	db := workloads[0]
	if db.Name != "db" || db.Namespace != "production" {
		t.Errorf("expected workloads[0] = production/db, got %s/%s", db.Namespace, db.Name)
	}
	if db.Replicas != 1 {
		t.Errorf("expected db replicas=1, got %d", db.Replicas)
	}
	if len(db.Containers) != 1 || db.Containers[0].Name != "db" {
		t.Errorf("expected db to have container 'db', got %v", db.Containers)
	}

	web := workloads[1]
	if web.Name != "web" || web.Namespace != "production" {
		t.Errorf("expected workloads[1] = production/web, got %s/%s", web.Namespace, web.Name)
	}
	if web.Replicas != 2 {
		t.Errorf("expected web replicas=2, got %d", web.Replicas)
	}
	if len(web.Containers) != 1 || web.Containers[0].Name != "app" {
		t.Errorf("expected web to have container 'app', got %v", web.Containers)
	}
	if web.Containers[0].Image != "nginx:1.25" {
		t.Errorf("expected image 'nginx:1.25', got %q", web.Containers[0].Image)
	}
	if web.Containers[0].CurrentRequest.CPUCores != 0.5 {
		t.Errorf("expected cpu_req=0.5, got %f", web.Containers[0].CurrentRequest.CPUCores)
	}
}

func TestGetWorkloadsFiltersNamespace(t *testing.T) {
	// Minimal server: dep in ns1 and ns2, nothing else.
	queryResponses := map[string][]byte{
		"kube_deployment_spec_replicas": promVectorResponse([]map[string]interface{}{
			{"namespace": "ns1", "deployment": "svc-a", "__value__": "1"},
			{"namespace": "ns2", "deployment": "svc-b", "__value__": "1"},
		}),
		"kube_statefulset_spec_replicas":                    promVectorResponse(nil),
		`kube_pod_owner{owner_kind="ReplicaSet"}`:           promVectorResponse(nil),
		`kube_replicaset_owner{owner_kind="Deployment"}`:    promVectorResponse(nil),
		`kube_pod_owner{owner_kind="StatefulSet"}`:          promVectorResponse(nil),
		`kube_pod_container_resource_requests{resource="cpu",container!=""}`:    promVectorResponse(nil),
		`kube_pod_container_resource_requests{resource="memory",container!=""}`: promVectorResponse(nil),
		`kube_pod_container_resource_limits{resource="cpu",container!=""}`:      promVectorResponse(nil),
		`kube_pod_container_resource_limits{resource="memory",container!=""}`:   promVectorResponse(nil),
		`kube_pod_container_info{container!=""}`:                                 promVectorResponse(nil),
	}
	emptyResp, err := json.Marshal(map[string]interface{}{
		"status": "success",
		"data":   map[string]interface{}{"resultType": "vector", "result": []interface{}{}},
	})
	if err != nil {
		t.Fatalf("failed to build empty response: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query" {
			query := r.URL.Query().Get("query")
			if resp, ok := queryResponses[query]; ok {
				w.Write(resp)
				return
			}
			w.Write(emptyResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	// GetWorkloads will call GetAllWorkloads and filter — both workloads have no
	// container data so the list will be empty, but the filtering path is exercised.
	workloads, err := c.GetWorkloads(context.Background(), "ns1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range workloads {
		if w.Namespace != "ns1" {
			t.Errorf("expected namespace ns1, got %s", w.Namespace)
		}
	}
}

func TestLookbackWindow(t *testing.T) {
	cases := []struct {
		lookback time.Duration
		want     string
	}{
		{168 * time.Hour, "7d"},
		{24 * time.Hour, "1d"},
		{12 * time.Hour, "12h"},
		{1 * time.Hour, "1h"},
	}
	for _, tc := range cases {
		c := NewClient("http://localhost:9090", tc.lookback)
		if got := c.LookbackWindow(); got != tc.want {
			t.Errorf("LookbackWindow(%v) = %q, want %q", tc.lookback, got, tc.want)
		}
	}
}

func TestQueryInstantError(t *testing.T) {
	srv := mockPromServer(t, map[string]http.HandlerFunc{
		"/api/v1/query": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"parse error"}`))
		},
	})
	defer srv.Close()

	c := NewClient(srv.URL, 168*time.Hour)
	_, err := c.queryInstant(context.Background(), "invalid{query")
	if err == nil {
		t.Fatal("expected error for bad query, got nil")
	}
}
