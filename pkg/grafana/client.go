package grafana

import (
	"errors"
	"net/url"
	"strings"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/k8s-auth-portal/pkg/random"
	log "github.com/sirupsen/logrus"
)

var (
	ErrRoleNotValid = errors.New("role is not valid")
)

type userOrgsRoleMap map[int64]models.RoleType

type Client struct {
	client GAPIClient
}

type GAPIClient interface {
	UserByEmail(email string) (user gapi.User, err error)
	CreateUser(user gapi.User) (int64, error)
	AddOrgUser(orgID int64, user, role string) error
	UpdateOrgUser(orgID, userID int64, role string) error
	UpdateUserPermissions(id int64, isAdmin bool) error
}

func NewClient(baseURL *url.URL, cfg gapi.Config) (*Client, error) {
	newClient := &Client{}

	if baseURL == nil {
		return nil, errors.New("url is nil")
	}

	client, err := gapi.New(baseURL.String(), cfg)
	if err != nil {
		return nil, err
	}

	newClient.client = client

	return newClient, nil
}

// LookupUser search for a user by Login or Email and returns it
func (c *Client) LookupUser(loginOrEmail string) (gapi.User, error) {
	user, err := c.client.UserByEmail(loginOrEmail)
	if err != nil {
		if !strings.Contains(err.Error(), "User not found") {
			return user, err
		}
	}

	return user, nil
}

// CreateUser adds a new global user to Grafana
func (c *Client) CreateUser(user gapi.User) (int64, error) {
	var uid int64

	// The Grafana API requires a password for user creation
	if user.Password == "" {
		// Generate new random password
		passwd, err := random.SecureString(12)
		if err != nil {
			log.Errorf("error generating random password: %v", err)

			return uid, err
		}

		user.Password = passwd
	}

	uid, err := c.client.CreateUser(user)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// AddOrgUser adds a user, with a role, to an Organization specified by OrgID
func (c *Client) AddOrgUser(OrgID int64, login string, role string) error {
	err := c.client.AddOrgUser(OrgID, login, role)
	if err != nil {
		return err
	}

	return nil
}

// UpsertOrgUser adds a user to an Organization if not present or
// updates the user role if already a member.
func (c *Client) UpsertOrgUser(orgID int64, user gapi.User, role string) error {

	var isOrgMember bool

	err := c.AddOrgUser(orgID, user.Login, role)
	if err != nil {
		if !strings.Contains(err.Error(), "User is already member of this organization") {
			return err
		} else {
			isOrgMember = true
		}
	}

	// Always update user even if it's a member as the roles might have changed.
	if isOrgMember {
		err = c.client.UpdateOrgUser(orgID, user.ID, role)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) UpdateUserPermissions(id int64, isAdmin bool) error {
	return c.client.UpdateUserPermissions(id, isAdmin)
}

// UpdateOrgUserAuthz updates both roles and global admin status for a user taking
// into account group configuration.
// it will return an error when there's an issue updating the GrafanaAdmin permissions
func (c *Client) UpdateOrgUserAuthz(user gapi.User, groups config.Groups) (userOrgsRoleMap, error) {
	// Mapping of role per org
	userOrgsRole := make(userOrgsRoleMap)

	for _, group := range groups {
		if group.GrafanaAdmin != user.IsAdmin {
			err := c.UpdateUserPermissions(user.ID, group.GrafanaAdmin)
			if err != nil {
				return userOrgsRole, err
			}
		}

		for _, org := range group.Orgs {
			// Check if the users has a more permissive role and apply that instead
			if !isRoleAssignable(userOrgsRole[org.ID], models.RoleType(org.Role)) {
				continue
			}

			userOrgsRole[org.ID] = models.RoleType(org.Role)
		}
	}

	return userOrgsRole, nil
}

func isRoleAssignable(currentRole models.RoleType, incomingRole models.RoleType) bool {
	// role hierarchy
	roleHierarchy := map[models.RoleType]int{
		models.ROLE_VIEWER: 0,
		models.ROLE_EDITOR: 1,
		models.ROLE_ADMIN:  2,
	}

	// If the incoming role is less than ( less privilege ) than the currently assigned role ( more privilege ), skip this mapping.
	if currentRole != "" && roleHierarchy[models.RoleType(incomingRole)] < roleHierarchy[currentRole] {
		return false
	}

	return true
}
