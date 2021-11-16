package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
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

		// Mapping of role per org
		userOrgsRole := make(map[int64]models.RoleType)

		if claims.Email != "" {
			orgUser.Email = claims.Email
		}

		foundUser, err := s.grafanaClient.LookupUser(claims.Subject)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error looking for user")
			return
		}

		if foundUser == nil {
			log.Infof("no user with login %s found, creating new one", orgUser.Login)
			uid, err := s.grafanaClient.CreateUser(orgUser)
			if err != nil {
				logAndError(w, http.StatusUnauthorized, err, "error creating new user")
				return
			}

			// Update ID on orgUser
			orgUser.ID = uid
		} else {
			orgUser.ID = foundUser.ID
		}

		if len(validUserGroups) > 0 {
			for _, group := range validUserGroups {
				if groupConfig, ok := s.groups[group]; ok {
					for _, org := range groupConfig.Orgs {
						// Check if the users has a more permissive role and apply that instead
						if !grafana.IsRoleAssignable(userOrgsRole[org.ID], models.RoleType(org.Role)) {
							continue
						}

						userOrgsRole[org.ID] = models.RoleType(org.Role)

						if org.GrafanaAdmin != nil && *org.GrafanaAdmin {
							orgUser.IsAdmin = true
						}
					}
				}
			}
		}

		for orgID, role := range userOrgsRole {
			err = s.grafanaClient.UpsertOrgUser(orgID, orgUser, string(role))
			if err != nil {
				logAndError(w, http.StatusUnauthorized, err, "error upserting user")
				return
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
