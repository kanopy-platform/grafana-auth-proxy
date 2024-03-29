package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
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

type GrafanaClaimsConfig struct {
	Login string
	Name  string
}

type Server struct {
	router                 *http.ServeMux
	cookieName             string
	headerName             string
	groups                 config.Groups
	grafanaProxyUrl        *url.URL
	grafanaClient          *grafana.Client
	grafanaResponseHeaders GrafanaResponseHeaders
	grafanaClaimsConfig    GrafanaClaimsConfig
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

	s.router.HandleFunc("/healthz", s.handleHealthz())
	s.router.HandleFunc("/", s.handleRoot())

	return s.router, nil
}

func getValidClaim(claims *jwt.Claims, input string) string {
	switch input {
	case "sub":
		return claims.Subject
	case "email":
		return claims.Email
	}
	return ""
}

func (s *Server) handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token
		var token string

		if s.headerName != "" {
			token = r.Header.Get(s.headerName)
			// replicate the cookie look up behavior for a missing header
			if token == "" {
				logAndError(w, http.StatusUnauthorized, fmt.Errorf("No value for header %s", s.headerName), "error reading header")
				return
			}
		} else {
			cookie, err := r.Cookie(s.cookieName)
			if err != nil {
				logAndError(w, http.StatusUnauthorized, err, "error reading cookie")
				return
			}
			token = cookie.Value
		}

		// Get claims from token
		claims, err := jwt.TokenClaims(token)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error reading claims from jwt token")
			return
		}

		if claims.Subject == "" {
			logAndError(w, http.StatusUnauthorized, err, "sub claim is empty")
			return
		}

		// possible values of Login claim are checked in cli beforehand
		login := getValidClaim(claims, s.grafanaClaimsConfig.Login)
		name := getValidClaim(claims, s.grafanaClaimsConfig.Name)
		email := claims.Email

		log.Infof("user %s is attempting to log in", login)
		log.Debugf("claim groups for user %s: %v", login, claims.Groups)

		// validUserGroups represents the intersection of user groups from claim with the group
		// mapping in configuration
		validUserGroups := config.ValidUserGroups(claims.Groups, s.groups)
		log.Debugf("valid user groups for user %s: %v", login, validUserGroups)

		orgUser, err := s.grafanaClient.GetOrCreateUser(login, name, email)
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
				log.Infof("err: %v", err)
				log.Infof("failed to update role %s in orgID %d for user %s", string(role), orgID, login)
			}
		}

		log.Infof("user %s is authorized to log in", login)

		r.Header.Set("X-Forwarded-Host", r.Host)
		r.Header.Set(s.grafanaResponseHeaders.User, login)

		// Remove the Authorization header as it's not needed anymore and will conflict with Grafana's API access
		r.Header.Del("Authorization")

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

func (s *Server) handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{
			"status": "ok",
		}

		bytes, err := json.Marshal(status)
		if err != nil {
			logAndError(w, http.StatusBadRequest, err, "error gathering status")
			return
		}
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprint(w, string(bytes))
	}
}

func logAndError(w http.ResponseWriter, code int, err error, msg string) {
	log.WithError(err).Error(msg)
	http.Error(w, http.StatusText(code), code)
}
