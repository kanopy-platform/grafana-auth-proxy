package server

import (
	"net/url"
	"strings"

	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
)

func WithCookieName(cookie string) ServerFuncOpt {
	return func(s *Server) error {
		s.cookieName = cookie
		return nil
	}
}

// WithHeaderName appends a single header name to the ordered list of headers
// consulted when extracting a JWT token. It is kept for backwards compatibility;
// prefer WithHeaderNames when configuring multiple headers.
func WithHeaderName(header string) ServerFuncOpt {
	return func(s *Server) error {
		if header = strings.TrimSpace(header); header != "" {
			s.headerNames = append(s.headerNames, header)
		}
		return nil
	}
}

// WithHeaderNames sets (replaces) the ordered list of header names to consult
// when extracting a JWT token. The first header that contains a non-empty value
// is used; the Bearer scheme is stripped automatically so both bare-JWT headers
// and standard Authorization headers work. Empty and whitespace-only entries
// are silently dropped.
func WithHeaderNames(headers []string) ServerFuncOpt {
	return func(s *Server) error {
		filtered := make([]string, 0, len(headers))
		for _, h := range headers {
			if h = strings.TrimSpace(h); h != "" {
				filtered = append(filtered, h)
			}
		}
		s.headerNames = filtered
		return nil
	}
}

func WithConfigGroups(groups config.Groups) ServerFuncOpt {
	return func(s *Server) error {
		s.groups = groups
		return nil
	}
}

func WithGrafanaProxyURL(grafanaUrl *url.URL) ServerFuncOpt {
	return func(s *Server) error {
		s.grafanaProxyUrl = grafanaUrl
		return nil
	}
}

func WithGrafanaClient(grafanaClient *grafana.Client) ServerFuncOpt {
	return func(s *Server) error {
		s.grafanaClient = grafanaClient
		return nil
	}
}

func SkipTLSVerify() ServerFuncOpt {
	return func(s *Server) error {
		s.skipTLSVerify = true
		return nil
	}
}

func WithGrafanaResponseHeaders(headers GrafanaResponseHeaders) ServerFuncOpt {
	return func(s *Server) error {
		s.grafanaResponseHeaders = headers
		return nil
	}
}

func WithGrafanaClaimsConfig(config GrafanaClaimsConfig) ServerFuncOpt {
	return func(s *Server) error {
		s.grafanaClaimsConfig = config
		return nil
	}
}
