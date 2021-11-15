package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	log "github.com/sirupsen/logrus"
)

const (
	grafanaAuthHeader = "X-WEBAUTH-USER"
)

type Server struct {
	router          *http.ServeMux
	cookieName      string
	groups          config.Groups
	grafanaProxyUrl *url.URL
	grafanaClient   *grafana.Client
	skipTLSVerify   bool
}

type ServerFuncOpt func(*Server) error

func New(opts ...ServerFuncOpt) (http.Handler, error) {
	s := &Server{router: http.NewServeMux()}

	// load options
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

	s.router.HandleFunc("/", s.handleRoot())

	return s.router, nil
}

func (s *Server) handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from cookie
		cookie, err := r.Cookie(s.cookieName)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error reading cookie")
			return
		}

		// Get claims from token
		claims, err := jwt.TokenClaims(cookie.Value)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error reading claims from jwt token")
			return
		}

		log.Infof("attempting to log in user %s", claims.Subject)
		log.Debugf("claim groups for user %s: %v", claims.Subject, claims.Groups)

		// validUserGroups represents the intersection of user groups from claim with the group
		// mapping in configuration
		validUserGroups := config.UserGroupsInConfig(claims.Groups, s.groups)
		log.Debugf("valid user groups for user %s: %v", claims.Subject, validUserGroups)

		// Creat gapi.User for lookup purposes
		orgUser := gapi.User{
			Login: claims.Subject,
		}

		if len(validUserGroups) > 0 {
			for _, group := range validUserGroups {
				data := s.groups[group]

				for _, org := range data.Orgs {
					err = s.grafanaClient.UpsertOrgUser(org.OrgId, orgUser, org.Role)
					if err != nil {
						logAndError(w, http.StatusUnauthorized, err, "error upserting user")
						return
					}
				}
			}
		}

		r.Header.Set("X-Forwarded-Host", r.Host)
		r.Header.Set(grafanaAuthHeader, claims.Subject)

		// Create the reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(s.grafanaProxyUrl)

		if s.skipTLSVerify {
			proxy.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
		}

		proxy.ServeHTTP(w, r)
	}
}

func logAndError(w http.ResponseWriter, code int, err error, msg string) {
	log.WithError(err).Error(msg)
	http.Error(w, http.StatusText(code), code)
}
