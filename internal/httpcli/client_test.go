// Verifies URL construction without sending query strings as escaped paths.
// Both Core bearer tokens and Proxmox authentication remain supported.
package httpcli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPreservesBasePathAndQuery(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth AuthType
		want string
	}{
		{"Core", AuthTypeBearer, "Bearer test-token"},
		{"Proxmox", AuthTypePVE, "PVEAPIToken=test-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/gateway/api/v1/services", r.URL.Path)
				assert.Equal(t, "2", r.URL.Query().Get("page"))
				assert.Equal(t, "test", r.URL.Query().Get("scope"))
				assert.Equal(t, []string{"a", "b"}, r.URL.Query()["agentId"])
				assert.Equal(t, tc.want, r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			client := NewHTTPClient(server.URL+"/gateway?scope=test&page=1", "test-token", WithAuthType(tc.auth))
			response, err := client.Get("/api/v1/services?page=2&agentId=a&agentId=b")
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

func TestGetRejectsInvalidURL(t *testing.T) {
	_, err := NewHTTPClient("://bad-url", "test").Get("/services")
	require.Error(t, err)
	_, err = NewHTTPClient("http://localhost", "test").Get("/%zz")
	require.Error(t, err)
}
