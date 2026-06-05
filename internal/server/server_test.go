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
	log "github.com/sirupsen/logrus"
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

// newServiceAccountJWTToken creates a JWT with only a sub claim, simulating a client credentials token.
func newServiceAccountJWTToken(subject string) string {
	cl := jwt.Claims{}
	cl.Subject = subject

	tokenString, _ := jwt.NewTestJWTWithClaims(cl)

	return tokenString
}

func TestTokenValidations(t *testing.T) {
	// the backendServer represents the Grafana server. In this case we are mocking the Grafana api calls
	// so the backend server is only here to avoid the proxy to timeout
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello, client")
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

func TestMultiHeaderNames(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello, client")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	aliceToken := newTestJWTToken("alice")
	bobToken := newTestJWTToken("bob")
	invalidToken := "this-is-no-valid-jwt"

	clientAlice := grafana.NewMockClient(gapi.User{Login: "alice", ID: 1}, map[int64]grafana.RoleType{})
	clientBob := grafana.NewMockClient(gapi.User{Login: "bob", ID: 2}, map[int64]grafana.RoleType{})

	baseOpts := func(client *grafana.Client) []ServerFuncOpt {
		return []ServerFuncOpt{
			WithGrafanaProxyURL(backendURL),
			WithConfigGroups(config.Groups{}),
			WithGrafanaClient(client),
			WithGrafanaResponseHeaders(GrafanaResponseHeaders{User: "X-WEBAUTH-USER"}),
			WithHeaderNames([]string{"X-Test-Header", "Authorization"}),
		}
	}

	t.Run("first header present and valid — identity from first token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Test-Header", aliceToken)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "alice", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("second header present and valid bare JWT — identity from second token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", bobToken)
		s, err := New(baseOpts(clientBob)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "bob", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("second header with Bearer prefix stripped — identity from second token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+bobToken)
		s, err := New(baseOpts(clientBob)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "bob", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("lowercase bearer prefix stripped — identity from second token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "bearer "+bobToken)
		s, err := New(baseOpts(clientBob)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "bob", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("first header wins when both present — identity is first token's subject", func(t *testing.T) {
		// alice in first header, bob (with Bearer) in second: alice must win.
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Test-Header", aliceToken)
		req.Header.Set("Authorization", "Bearer "+bobToken)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "alice", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("first header present but invalid does not fall through to second header", func(t *testing.T) {
		// Priority is positional: first non-empty header value is used even if the JWT is invalid.
		// This prevents an attacker supplying a valid Authorization header from bypassing a
		// rejected, alternative token.
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Test-Header", invalidToken)
		req.Header.Set("Authorization", "Bearer "+aliceToken)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("no configured headers present returns unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("bare JWT header has no Bearer prefix — extractToken is a no-op", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Test-Header", aliceToken)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "alice", req.Header.Get("X-WEBAUTH-USER"))
	})

	t.Run("configured auth headers are removed before proxying", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Test-Header", aliceToken)
		req.Header.Set("Authorization", "Bearer "+bobToken)
		s, err := New(baseOpts(clientAlice)...)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "", req.Header.Get("X-Test-Header"), "X-Test-Header should be removed before proxying")
		assert.Equal(t, "", req.Header.Get("Authorization"), "Authorization should be removed before proxying")
	})

	t.Run("empty entries in WithHeaderNames do not disable cookie fallback", func(t *testing.T) {
		// WithHeaderNames([]string{""}) should produce an empty headerNames slice,
		// so the server falls back to cookies as if no headers were configured.
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: aliceToken})
		s, err := New(
			WithGrafanaProxyURL(backendURL),
			WithConfigGroups(config.Groups{}),
			WithGrafanaClient(clientAlice),
			WithGrafanaResponseHeaders(GrafanaResponseHeaders{User: "X-WEBAUTH-USER"}),
			WithCookieName("auth_token"),
			WithHeaderNames([]string{"", "  ", ""}), // all empty/whitespace — should be filtered
		)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "alice", req.Header.Get("X-WEBAUTH-USER"))
	})
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"bare JWT unchanged", "eyJhbGc.eyJzdW.sig", "eyJhbGc.eyJzdW.sig"},
		{"Bearer prefix stripped", "Bearer eyJhbGc.eyJzdW.sig", "eyJhbGc.eyJzdW.sig"},
		{"lowercase bearer stripped", "bearer eyJhbGc.eyJzdW.sig", "eyJhbGc.eyJzdW.sig"},
		{"mixed case Bearer stripped", "BEARER eyJhbGc.eyJzdW.sig", "eyJhbGc.eyJzdW.sig"},
		{"extra space after Bearer stripped", "Bearer  eyJhbGc.eyJzdW.sig", "eyJhbGc.eyJzdW.sig"},
		{"leading/trailing whitespace trimmed", "  Bearer eyJhbGc.eyJzdW.sig  ", "eyJhbGc.eyJzdW.sig"},
		{"bare JWT with surrounding whitespace trimmed", "  eyJhbGc.eyJzdW.sig  ", "eyJhbGc.eyJzdW.sig"},
		{"Basic scheme not stripped", "Basic dXNlcjpwYXNz", "Basic dXNlcjpwYXNz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractToken(tt.input))
		})
	}
}

func TestHandleRoot(t *testing.T) {
	t.Parallel()

	// the backendServer represents the Grafana server. In this case we are mocking the Grafana api calls
	// so the backend server is only here to avoid the proxy to timeout
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello, client")
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

func TestSubLoginFallback(t *testing.T) {
	t.Parallel()

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello, client")
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	// Mock client expects the sub value as the login identifier
	client := grafana.NewMockClient(gapi.User{Login: "svc-account-sub", ID: 1}, map[int64]grafana.RoleType{})

	server, err := New(
		WithGrafanaProxyURL(backendURL),
		WithCookieName("auth_token"),
		WithConfigGroups(config.Groups{}),
		WithGrafanaClient(client),
		WithGrafanaResponseHeaders(GrafanaResponseHeaders{User: "X-WEBAUTH-USER"}),
		// Default login claim is "email"; token has no email so sub should be used
		WithGrafanaClaimsConfig(GrafanaClaimsConfig{Login: "email", Name: "sub"}),
	)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "http://grafana.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: newServiceAccountJWTToken("svc-account-sub"),
	})

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "svc-account-sub", req.Header.Get("X-WEBAUTH-USER"))
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

// testHook is a logrus hook that captures log entries for testing
type testHook struct {
	entries []*log.Entry
}

func (h *testHook) Levels() []log.Level {
	return []log.Level{log.InfoLevel, log.ErrorLevel, log.DebugLevel}
}

func (h *testHook) Fire(entry *log.Entry) error {
	h.entries = append(h.entries, entry)
	return nil
}

func TestRequestLogging(t *testing.T) {
	// Set up a test hook to capture log entries
	hook := &testHook{entries: []*log.Entry{}}
	log.AddHook(hook)
	defer func() {
		// Clean up - remove the hook by resetting the standard logger
		log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	}()

	// Create a backend server that returns different status codes
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources/1":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"id": 1}`)
		case "/api/dashboards/notfound":
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintln(w, `{"message": "not found"}`)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Hello, client")
		}
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

	client := grafana.NewMockClient(gapi.User{Login: "testuser", ID: 1}, orgRoleMap)

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

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedUser   string
		expectedEmail  string
		expectedGroups []string
	}{
		{
			name:           "GET request to datasource",
			method:         "GET",
			path:           "/api/datasources/1",
			expectedStatus: http.StatusOK,
			expectedUser:   "testuser",
			expectedEmail:  "testuser@example.com",
			expectedGroups: []string{"foo", "bar"},
		},
		{
			name:           "POST request with 404 response",
			method:         "POST",
			path:           "/api/dashboards/notfound",
			expectedStatus: http.StatusNotFound,
			expectedUser:   "testuser",
			expectedEmail:  "testuser@example.com",
			expectedGroups: []string{"foo", "bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear previous log entries
			hook.entries = []*log.Entry{}

			req := httptest.NewRequest(test.method, test.path, nil)
			req.Host = "http://grafana.example.com"
			req.AddCookie(&http.Cookie{
				Name:  "auth_token",
				Value: newTestJWTToken("testuser"),
			})

			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			assert.Equal(t, test.expectedStatus, w.Code)

			// Find the log entry for "request proxied to Grafana"
			var logEntry *log.Entry
			for _, entry := range hook.entries {
				if entry.Message == "request proxied to grafana" {
					logEntry = entry
					break
				}
			}

			assert.NotNil(t, logEntry, "Expected log entry 'request proxied to Grafana' not found")
			if logEntry == nil {
				return
			}

			// Verify all required fields are present (using the changed field names)
			assert.Equal(t, test.method, logEntry.Data["method"], "method field should match")
			assert.Equal(t, test.path, logEntry.Data["path"], "path field should match")
			assert.Equal(t, test.expectedStatus, logEntry.Data["status"], "status field should match")
			assert.Equal(t, test.expectedEmail, logEntry.Data["user_email"], "user_email field should match")
			assert.Equal(t, "testuser", logEntry.Data["user_sub"], "user_sub field should match")
		})
	}
}
