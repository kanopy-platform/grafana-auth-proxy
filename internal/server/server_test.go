package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	"github.com/stretchr/testify/assert"
)

func TestHandleRoot(t *testing.T) {
	t.Parallel()

	groups := config.Groups{
		"first": {
			Orgs: []config.Org{
				{
					ID:   1,
					Role: "Viewer",
				},
			},
		},
	}

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the reverse proxy")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	client, _ := grafana.NewClient(backendURL, gapi.Config{})

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "http://grafana.example.com"

	// Token Payload data
	// {
	// 	"sub": "jhon",
	// 	"name": "John Doe",
	// 	"email": "jhon@example.com",
	// 	"groups": ["foo", "bar"]
	// }
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJqaG9uIiwibmFtZSI6IkpvaG4gRG9lIiwiZW1haWwiOiJqaG9uQGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbImZvbyIsImJhciJdfQ.gr20bPyDDFvmqV9ec71HFB7-c2iACkIoY-0VDXP_9DY",
	})

	server, err := New(
		WithGrafanaProxyURL(backendURL),
		WithCookieName("auth_token"),
		WithConfigGroups(groups),
		WithGrafanaClient(client),
	)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
