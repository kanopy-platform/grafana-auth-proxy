package server

import (
	"net/url"

	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
)

func WithCookieName(cookie string) ServerFuncOpt {
	return func(s *Server) error {
		s.cookieName = cookie
		return nil
	}
}

func WithConfig(config *config.Config) ServerFuncOpt {
	return func(s *Server) error {
		s.config = config
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
