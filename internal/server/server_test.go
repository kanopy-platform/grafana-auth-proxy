package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	"github.com/stretchr/testify/assert"
)

func TestHandleRoot(t *testing.T) {
	t.Parallel()

	groups := config.Groups{
		"foo": {
			Orgs: []config.Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
	}

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-WEBAUTH-USER") == "jhon" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	orgRoleMap := map[int64]models.RoleType{
		1: models.ROLE_EDITOR,
	}

	client := grafana.NewMockClient(gapi.User{Login: "jhon"}, orgRoleMap)

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
		WithGrafanaResponseHeaders(GrafanaResponseHeaders{
			User: "X-WEBAUTH-USER",
		}),
	)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
