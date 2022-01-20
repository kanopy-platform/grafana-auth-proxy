package jwt

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	cookieName = "auth_token"
	headerName = "Authorization"
)

func setupToken(t *testing.T) string {
	cl := Claims{}
	cl.Subject = "jhon.doe"

	token, err := NewTestJWTWithClaims(cl)
	assert.NoError(t, err)

	return token
}

func setupRequestWithCookie(token string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: token,
	})
	return req
}

func setupRequestWithHeader(token string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(headerName, fmt.Sprintf("Bearer %s", token))
	return req
}

func TestCookieContainer(t *testing.T) {
	token := setupToken(t)
	req := setupRequestWithCookie(token)

	container := NewCookieContainer(cookieName)
	got, err := container.Get(req)
	assert.NoError(t, err)
	assert.Equal(t, got, token)
}

func TestHeaderContainer(t *testing.T) {
	token := setupToken(t)
	req := setupRequestWithHeader(token)

	container := NewHeaderContainer(headerName)
	got, err := container.Get(req)
	assert.NoError(t, err)
	assert.Equal(t, got, token)
}

func TestGetFirstFromContainers(t *testing.T) {
	token := setupToken(t)

	tests := []struct {
		containers []TokenContainer
		withCookie bool
		withHeader bool
		want       string
		err        error
	}{
		// With Cookie only
		{
			containers: []TokenContainer{
				NewCookieContainer(cookieName),
			},
			withCookie: true,
			want:       token,
			err:        nil,
		},
		// With Header only
		{
			containers: []TokenContainer{
				NewHeaderContainer(headerName),
			},
			withHeader: true,
			want:       token,
			err:        nil,
		},
		// values in both containers
		{
			containers: []TokenContainer{
				NewCookieContainer(cookieName),
				NewHeaderContainer(headerName),
			},
			withCookie: true,
			withHeader: true,
			want:       token,
			err:        nil,
		},
		// value in only one container
		{
			containers: []TokenContainer{
				NewCookieContainer(cookieName),
				NewHeaderContainer(headerName),
			},
			withCookie: false,
			withHeader: true,
			want:       token,
			err:        nil,
		},
		// failure in both containers, error comes from last container that failed
		{
			containers: []TokenContainer{
				NewCookieContainer(cookieName),
				NewHeaderContainer(headerName),
			},
			withCookie: false,
			withHeader: false,
			want:       "",
			err:        fmt.Errorf("no header %s found", headerName),
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		if test.withCookie {
			req.AddCookie(&http.Cookie{
				Name:  cookieName,
				Value: token,
			})
		}
		if test.withHeader {
			req.Header.Add(headerName, fmt.Sprintf("Bearer %s", token))
		}

		result, err := GetFirstFromContainers(req, test.containers)
		assert.Equal(t, err, test.err)
		assert.Equal(t, test.want, result)
	}
}
