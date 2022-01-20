package jwt

import (
	"fmt"
	"net/http"
	"strings"
)

type TokenContainer interface {
	Get(req *http.Request) (string, error)
}

type CookieContainer struct {
	cookieName string
}

func NewCookieContainer(cookieName string) *CookieContainer {
	return &CookieContainer{cookieName: cookieName}
}

func (c *CookieContainer) Get(req *http.Request) (string, error) {
	cookie, err := req.Cookie(c.cookieName)
	if err != nil {
		return "", err
	}

	if cookie.Value == "" {
		return "", fmt.Errorf("cookie %s has empty value", cookie.Name)
	}

	return cookie.Value, nil
}

type HeaderContainer struct {
	headerName string
}

func NewHeaderContainer(headerName string) *HeaderContainer {
	return &HeaderContainer{headerName: headerName}
}

func (h *HeaderContainer) Get(req *http.Request) (string, error) {
	value := req.Header.Get(h.headerName)

	// strip Bearer prefix if present
	value = strings.TrimPrefix(value, "Bearer ")

	if value == "" {
		return "", fmt.Errorf("no header %s found", h.headerName)
	}

	return value, nil
}

func GetFirstFromContainers(req *http.Request, containers []TokenContainer) (string, error) {
	var token string
	var err error

	for _, container := range containers {
		token, err = container.Get(req)
		if token != "" {
			break
		}
	}

	return token, err
}
