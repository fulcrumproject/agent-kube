// Tests the Fulcrum HTTP contract against the current Core API.
// All requests use local HTTP servers and synthetic data.
package fulcrum

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"fulcrumproject.org/kube-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc) *HTTPFulcrumClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return NewFulcrumClient(server.URL, "test-token")
}

func TestGetPendingJobsContract(t *testing.T) {
	const payload = `[{
		"id":"job-1","serviceId":"service-1","action":"update","status":"Pending",
		"params":{"nodes":[{"id":"node2","size":"s2","status":"On"}]},
		"service":{
			"id":"service-1","name":"cluster","status":"Running",
			"properties":{"nodes":[{"id":"node1","size":"s1","status":"On"}]},
			"agentInstanceId":"cluster-1",
			"agentInstanceData":{"nodes":{"node1":1001},"clusterIp":"https://cluster"}
		}
	}]`
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/jobs/pending", r.URL.Path)
		_, _ = io.WriteString(w, payload)
	})
	jobs, err := client.GetPendingJobs()
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	encoded, err := json.Marshal(jobs)
	require.NoError(t, err)
	var actual, expected []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &actual))
	require.NoError(t, json.Unmarshal([]byte(payload), &expected))
	for _, key := range []string{"serviceId", "action", "params"} {
		assert.Equal(t, expected[0][key], actual[0][key], key)
	}
	gotService := actual[0]["service"].(map[string]any)
	wantService := expected[0]["service"].(map[string]any)
	for _, key := range []string{"status", "properties", "agentInstanceId", "agentInstanceData"} {
		assert.Equal(t, wantService[key], gotService[key], key)
	}
}

func TestGetServicesPagination(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/services", r.URL.Path)
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		_, _ = io.WriteString(w, `{"items":[],"currentPage":2,"totalPages":2,"hasNext":false}`)
	})
	page, err := client.GetServices(2)
	require.NoError(t, err)
	require.Equal(t, 2, page.CurrentPage)
	require.False(t, page.HasNext)
}

func TestCompleteJobContract(t *testing.T) {
	const payload = `{"agentInstanceId":"cluster-1","agentInstanceData":{"nodes":{"node1":1001}}}`
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/jobs/job-1/complete", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, payload, string(body))
		w.WriteHeader(http.StatusNoContent)
	})
	var response agent.JobResponse
	require.NoError(t, json.Unmarshal([]byte(payload), &response))
	require.NoError(t, client.CompleteJob("job-1", response))
}

func TestReportMetricContract(t *testing.T) {
	const payload = `{"agentInstanceId":"cluster-1","resourceId":"node1","value":0.25,"typeName":"vm.cpu.usage"}`
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/metric-entries", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, payload, string(body))
		w.WriteHeader(http.StatusCreated)
	})
	var metric agent.MetricEntry
	require.NoError(t, json.Unmarshal([]byte(payload), &metric))
	require.NoError(t, client.ReportMetric(&metric))
}

func TestStructuredErrorMessage(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"status":400,"message":"invalid properties","errors":[{"path":"nodes","message":"required"}]}`)
	})
	err := client.CompleteJob("job-1", agent.JobResponse{})
	require.ErrorContains(t, err, "400")
	require.ErrorContains(t, err, "invalid properties")
	require.ErrorContains(t, err, "nodes: required")
}

func TestAllEndpointsPreserveStructuredErrors(t *testing.T) {
	operations := map[string]func(*HTTPFulcrumClient) error{
		"status":       func(c *HTTPFulcrumClient) error { return c.UpdateAgentStatus("Connected") },
		"agent info":   func(c *HTTPFulcrumClient) error { _, err := c.GetAgentInfo(); return err },
		"pending jobs": func(c *HTTPFulcrumClient) error { _, err := c.GetPendingJobs(); return err },
		"claim":        func(c *HTTPFulcrumClient) error { return c.ClaimJob("job-1") },
		"complete":     func(c *HTTPFulcrumClient) error { return c.CompleteJob("job-1", agent.JobResponse{}) },
		"fail":         func(c *HTTPFulcrumClient) error { return c.FailJob("job-1", "test failure") },
		"metric":       func(c *HTTPFulcrumClient) error { return c.ReportMetric(&agent.MetricEntry{}) },
		"services":     func(c *HTTPFulcrumClient) error { _, err := c.GetServices(1); return err },
		"service":      func(c *HTTPFulcrumClient) error { _, err := c.GetService("service-1"); return err },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"status":403,"message":"access denied","template":"access {reason}","data":{"reason":"denied"}}`)
			})
			err := operation(client)
			var apiErr *APIError
			require.True(t, errors.As(err, &apiErr), "%v", err)
			assert.Equal(t, http.StatusForbidden, apiErr.Status)
			assert.Equal(t, "access denied", apiErr.Message)
			assert.Equal(t, "access {reason}", apiErr.Template)
			assert.Equal(t, "denied", apiErr.Data["reason"])
		})
	}
}

func TestNonJSONErrorFallback(t *testing.T) {
	for _, body := range []string{"", "<html>proxy error</html>", `{"message":`} {
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, body)
		})
		err := client.ClaimJob("job-1")
		require.ErrorContains(t, err, "HTTP 502: Bad Gateway")
	}
}

func TestGetServiceLifecycle(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/services/service-1", r.URL.Path)
		_, _ = io.WriteString(w, `{"id":"service-1","status":"Active","serviceType":{"lifecycleSchema":{"runningStates":["Active"]}}}`)
	})
	service, err := client.GetService("service-1")
	require.NoError(t, err)
	running, err := service.IsRunning()
	require.NoError(t, err)
	require.True(t, running)
}

func TestCoreCreateJobRoundTrip(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	var completed map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/jobs/pending":
			_, _ = io.WriteString(w, `[{"id":"job-1","serviceId":"service-1","action":"create","status":"Pending","params":{"nodes":[{"id":"node1","size":"s1","status":"On"}]},"service":{"id":"service-1","name":"cluster","status":"New","properties":{"nodes":[]}}}]`)
		case "/api/v1/jobs/job-1/claim":
			w.WriteHeader(http.StatusNoContent)
		case "/api/v1/jobs/job-1/complete":
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&completed))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	proxmox := agent.NewMockProxmoxClient("test-node")
	proxmox.AddVM(100, "template", agent.VMStatusStopped, 2, 2048)
	handler := agent.NewJobHandler(client, proxmox, 100, "test", agent.NewMockKamajiClient(), agent.NewMockSSHClient())
	require.NoError(t, handler.PollAndProcessJobs())
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"GET /api/v1/jobs/pending", "POST /api/v1/jobs/job-1/claim", "POST /api/v1/jobs/job-1/complete"}, requests)
	require.Equal(t, "cluster-cluster", completed["agentInstanceId"])
	require.NotContains(t, completed, "resources")
	require.NotContains(t, completed, "externalId")
	data, ok := completed["agentInstanceData"].(map[string]any)
	require.True(t, ok)
	nodes, ok := data["nodes"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, nodes, "node1", "the handler must use job params, not service properties")
	require.NotEmpty(t, data["clusterIp"])
	require.NotEmpty(t, data["kubeConfig"])
}
