package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
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

	// the backendServer represents the Grafana server. In this case we are mocking the Grafana api calls
	// so the backend server is only here to avoid the proxy to timeout
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	orgRoleMap := map[int64]models.RoleType{
		1: models.ROLE_EDITOR,
	}

	client := grafana.NewMockClient(gapi.User{Login: "jhon"}, orgRoleMap)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "http://grafana.example.com"

	cl := jwt.Claims{
		Email:  "jhon@example.com",
		Groups: []string{"foo", "bar"},
	}
	cl.Subject = "jhon"

	tokenString, err := jwt.NewTestJWTWithClaims(cl)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: tokenString,
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
	assert.Equal(t, "http://grafana.example.com", req.Header.Get("X-Forwarded-Host"))
	assert.Equal(t, "jhon", req.Header.Get("X-WEBAUTH-USER"))
	assert.Equal(t, http.StatusOK, w.Code)
}
