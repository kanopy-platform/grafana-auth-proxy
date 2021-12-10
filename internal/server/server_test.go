package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	"github.com/stretchr/testify/assert"
)

func newTestJWTToken(subject string) string {
	cl := jwt.Claims{
		Email:  fmt.Sprintf("%s@example.com", subject),
		Groups: []string{"foo", "bar"},
	}
	cl.Subject = subject

	tokenString, _ := jwt.NewTestJWTWithClaims(cl)

	return tokenString
}

func TestTokenValidations(t *testing.T) {
	// the backendServer represents the Grafana server. In this case we are mocking the Grafana api calls
	// so the backend server is only here to avoid the proxy to timeout
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	client := grafana.NewMockClient(&gapi.User{Login: "jhon", ID: 1}, map[int64]models.RoleType{})

	tests := []struct {
		name       string
		cookie     *http.Cookie
		authorized bool
	}{
		{
			name: "valid JWT token and cookie",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: newTestJWTToken("jhon"),
			},
			authorized: true,
		},
		{
			name: "valid JWT token without sub",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: newTestJWTToken(""),
			},
		},
		{
			name: "invalid JWT token",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: "this-is-no-valid-jwt",
			},
		},
		{
			name: "invalid cookie",
			cookie: &http.Cookie{
				Name: "",
			},
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "http://grafana.example.com"

		if test.cookie.Name != "" {
			req.AddCookie(test.cookie)
		}

		server, err := New(
			WithGrafanaProxyURL(backendURL),
			WithCookieName(test.cookie.Name),
			WithConfigGroups(config.Groups{}),
			WithGrafanaClient(client),
			WithGrafanaResponseHeaders(GrafanaResponseHeaders{
				User: "X-WEBAUTH-USER",
			}),
		)
		assert.NoError(t, err)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if test.authorized {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}
	}
}

func TestHandleRoot(t *testing.T) {
	t.Parallel()

	// the backendServer represents the Grafana server. In this case we are mocking the Grafana api calls
	// so the backend server is only here to avoid the proxy to timeout
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

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

	orgRoleMap := map[int64]models.RoleType{
		1: models.ROLE_EDITOR,
	}

	client := grafana.NewMockClient(&gapi.User{Login: "jhon", ID: 1}, orgRoleMap)

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
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "http://grafana.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: newTestJWTToken("jhon"),
	})

	server.ServeHTTP(w, req)
	assert.Equal(t, "http://grafana.example.com", req.Header.Get("X-Forwarded-Host"))
	assert.Equal(t, "jhon", req.Header.Get("X-WEBAUTH-USER"))
	assert.Equal(t, http.StatusOK, w.Code)
}
