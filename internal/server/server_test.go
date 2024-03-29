package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
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

	headerName := "X-Test-Header"

	validToken := newTestJWTToken("jhon")
	noSubToken := newTestJWTToken("")
	invalidToken := "this-is-no-valid-jwt"
	emptyToken := ""

	client := grafana.NewMockClient(gapi.User{Login: "jhon", ID: 1}, map[int64]grafana.RoleType{})

	tests := []struct {
		name       string
		cookie     *http.Cookie
		header     *string
		authorized bool
	}{
		{
			name: "valid JWT token and cookie",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: validToken,
			},
			authorized: true,
		},
		{
			name: "valid JWT token without sub",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: noSubToken,
			},
		},
		{
			name: "invalid JWT token",
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: invalidToken,
			},
		},
		{
			name: "invalid cookie",
			cookie: &http.Cookie{
				Name: "",
			},
		},
		{
			name:       "valid JWT token and header",
			header:     &validToken,
			authorized: true,
		},
		{
			name:   "valid JWT token without sub",
			header: &noSubToken,
		},
		{
			name:   "invalid JWT token",
			header: &invalidToken,
		},
		{
			name:   "Empty Header",
			header: &emptyToken,
		},
		{
			name:   "valid JWT token header and cookie",
			header: &validToken,
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: validToken,
			},
			authorized: true,
		},
		{
			name:   "valid JWT token header invalid cookie",
			header: &validToken,
			cookie: &http.Cookie{
				Name:  "auth_token",
				Value: invalidToken,
			},
			authorized: true,
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "http://grafana.example.com"

		opts := []ServerFuncOpt{
			WithGrafanaProxyURL(backendURL),
			WithConfigGroups(config.Groups{}),
			WithGrafanaClient(client),
			WithGrafanaResponseHeaders(GrafanaResponseHeaders{
				User: "X-WEBAUTH-USER",
			}),
		}

		if test.cookie != nil && test.cookie.Name != "" {
			req.AddCookie(test.cookie)
			opts = append(opts, WithCookieName(test.cookie.Name))
		}
		if test.header != nil {
			req.Header.Add(headerName, *test.header)
			opts = append(opts, WithHeaderName(headerName))
		}
		server, err := New(opts...)
		assert.NoError(t, err)

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if test.authorized {
			assert.Equal(t, http.StatusOK, w.Code, test.name)
		} else {
			assert.Equal(t, http.StatusUnauthorized, w.Code, test.name)
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

	orgRoleMap := map[int64]grafana.RoleType{
		1: grafana.ROLE_EDITOR,
	}

	client := grafana.NewMockClient(gapi.User{Login: "jhon", ID: 1}, orgRoleMap)

	server, err := New(
		WithGrafanaProxyURL(backendURL),
		WithCookieName("auth_token"),
		WithConfigGroups(groups),
		WithGrafanaClient(client),
		WithGrafanaResponseHeaders(GrafanaResponseHeaders{
			User: "X-WEBAUTH-USER",
		}),
		WithGrafanaClaimsConfig(GrafanaClaimsConfig{
			Login: "sub",
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

	// Verify that if an Authorization header is present, it's correctly removed
	req.Header.Add("Authorization", "something")

	server.ServeHTTP(w, req)
	assert.Equal(t, "http://grafana.example.com", req.Header.Get("X-Forwarded-Host"))
	assert.Equal(t, "jhon", req.Header.Get("X-WEBAUTH-USER"))
	assert.Equal(t, "", req.Header.Get("Authorization"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleHealthz(t *testing.T) {
	server, err := New()
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	server.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	want := map[string]string{"status": "ok"}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(w.Result().Body)
	assert.NoError(t, err)

	got := map[string]string{}
	err = json.Unmarshal(buf.Bytes(), &got)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetValidClaim(t *testing.T) {
	claims := &jwt.Claims{
		Email: fmt.Sprintf("%s@example.com", "jhon.doe"),
	}
	claims.Subject = "jhon.doe"

	tests := []struct {
		claimKey string
		expected string
	}{
		{
			claimKey: "sub",
			expected: claims.Subject,
		},
		{
			claimKey: "email",
			expected: claims.Email,
		},
	}

	for _, test := range tests {
		value := getValidClaim(claims, test.claimKey)
		assert.Equal(t, test.expected, value)
	}
}
