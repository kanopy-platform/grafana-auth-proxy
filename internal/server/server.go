package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
	log "github.com/sirupsen/logrus"
)

type GrafanaResponseHeaders struct {
	User string
}

type Server struct {
	router                 *http.ServeMux
	cookieName             string
	groups                 config.Groups
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

		if len(claims.Groups) == 0 {
			logAndError(w, http.StatusUnauthorized, err, "groups claim is empty")
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
		if len(validUserGroups) == 0 {
			logAndError(w, http.StatusUnauthorized, err, "no user groups matching configured mapping")
			return
		}

		// lookup the user globally first as if it is not present it would need to
		// be created
		orgUser, err := s.grafanaClient.LookupUser(claims.Subject)
		if err != nil {
			logAndError(w, http.StatusUnauthorized, err, "error looking for user")
			return
		}

		// Assign the email from claim so it's always updated
		orgUser.Email = claims.Email

		// if the Login field in user is empty, it means that the user wasn't found
		if orgUser.Login == "" {
			log.Infof("no user with login %s found, creating new one", login)
			orgUser.Login = login

			uid, err := s.grafanaClient.CreateUser(orgUser)
			if err != nil {
				logAndError(w, http.StatusUnauthorized, err, "error creating new user")
				return
			}

			orgUser.ID = uid
		}

		// Mapping of role per org
		userOrgsRole := make(map[int64]models.RoleType)

		for _, groupConfig := range validUserGroups {
			if groupConfig.GrafanaAdmin != orgUser.IsAdmin {
				log.Infof("updating global admin status for user %s", login)
				err := s.grafanaClient.UpdateUserPermissions(orgUser.ID, groupConfig.GrafanaAdmin)
				if err != nil {
					logAndError(w, http.StatusUnauthorized, err, "error assigning the user as Grafana admin")
					return
				}
			}

			for _, org := range groupConfig.Orgs {
				// Check if the users has a more permissive role and apply that instead
				if !grafana.IsRoleAssignable(userOrgsRole[org.ID], models.RoleType(org.Role)) {
					continue
				}

				userOrgsRole[org.ID] = models.RoleType(org.Role)
			}
		}

		for orgID, role := range userOrgsRole {
			err = s.grafanaClient.UpsertOrgUser(orgID, orgUser, string(role))
			if err != nil {
				logAndError(w, http.StatusUnauthorized, err, "error upserting user")
				return
			}
		}

		log.Infof("user %s is authorized to log in", login)

		r.Header.Set("X-Forwarded-Host", r.Host)
		r.Header.Set(s.grafanaResponseHeaders.User, claims.Subject)

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
