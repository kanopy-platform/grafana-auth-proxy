package jwt

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieContainer(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	cl := Claims{}
	cl.Subject = "jhon.doe"

	token, _ := NewTestJWTWithClaims(cl)

	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})

	container := NewCookieContainer("auth_token")
	got, err := container.Get(req)
	assert.NoError(t, err)
	assert.Equal(t, got, token)
}

func TestHeaderContainer(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	cl := Claims{}
	cl.Subject = "jhon.doe"

	token, _ := NewTestJWTWithClaims(cl)

	req.Header.Add("Authorization", token)

	container := NewHeaderContainer("Authorization")
	got, err := container.Get(req)
	assert.NoError(t, err)
	assert.Equal(t, got, token)
}

func TestGetFirstFromContainers(t *testing.T) {
	cookieName := "auth_token"
	headerName := "Authorization"

	cl := Claims{}
	cl.Subject = "jhon.doe"
	token, _ := NewTestJWTWithClaims(cl)

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
			err:        errors.New("no header Authorization found"),
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
			req.Header.Add(headerName, token)
		}

		result, err := GetFirstFromContainers(req, test.containers)
		assert.Equal(t, err, test.err)
		assert.Equal(t, test.want, result)
	}
}
