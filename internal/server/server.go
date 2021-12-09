package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	log "github.com/sirupsen/logrus"
)

type GrafanaResponseHeaders struct {
	User string
}

type Server struct {
	router                 *http.ServeMux
	cookieName             string
	groups                 config.GroupsMap
	grafanaProxyUrl        *url.URL
	grafanaClient          *grafana.Client
	grafanaResponseHeaders GrafanaResponseHeaders
	skipTLSVerify          bool
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

		if claims.Subject == "" {
			logAndError(w, http.StatusUnauthorized, err, "sub claim is empty")
			return
		}

		// Make Subject claim the value used for login
		login := claims.Subject

		log.Infof("user %s is attempting to log in", login)
		log.Debugf("claim groups for user %s: %v", login, claims.Groups)

		// validUserGroups represents the intersection of user groups from claim with the group
		// mapping in configuration
		validUserGroups := config.ValidUserGroups(claims.Groups, s.groups)
		log.Debugf("valid user groups for user %s: %v", login, validUserGroups)

		orgUser, err := s.grafanaClient.GetOrCreateUser(login)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error obtaining or creating user")
			return
		}

		userOrgsRole, err := s.grafanaClient.UpdateOrgUserAuthz(orgUser, validUserGroups)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error updating global Grafana admin permissions")
			return
		}

		for orgID, role := range userOrgsRole {
			err = s.grafanaClient.UpsertOrgUser(orgID, orgUser, string(role))
			if err != nil {
				// if an upsert fails we still allow the user to login as it will be assigned to
				// the configured default Org and Role
				log.Infof("failed to update role %s in orgID %d for user %s", string(role), orgID, login)
			}
		}

		log.Infof("user %s is authorized to log in", login)

		r.Header.Set("X-Forwarded-Host", r.Host)
		r.Header.Set(s.grafanaResponseHeaders.User, login)

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
