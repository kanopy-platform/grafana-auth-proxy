package grafana

import (
	"errors"
	"net/url"
	"strings"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/k8s-auth-portal/pkg/random"
	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidURL           = errors.New("url is nil")
	ErrRoleNotValid         = errors.New("role is not valid")
	ErrUserNotFound         = errors.New("user not found")
	ErrOrgUserAlreadyMember = errors.New("user is already member of this organization")
)

type RoleType string

// Grafana pkgs cannot be safely imported as dependencies.
const (
	ROLE_VIEWER RoleType = "Viewer"
	ROLE_EDITOR RoleType = "Editor"
	ROLE_ADMIN  RoleType = "Admin"
)

type userOrgsRoleMap map[int64]RoleType

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
		return nil, ErrInvalidURL
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

		// the error message is longer but given that this is constrained
		// to a returned message from an specific call, it's better to keep it short
		if !strings.Contains(strings.ToLower(err.Error()), ErrUserNotFound.Error()) {
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

		if !strings.Contains(strings.ToLower(err.Error()), ErrOrgUserAlreadyMember.Error()) {
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

// UpdateOrgUserAuthz updates both roles and global admin status for a user
// taking into account group configuration. It outputs a mapping of role-in-org
// it will return an error when there's an issue updating the GrafanaAdmin permissions
func (c *Client) UpdateOrgUserAuthz(user gapi.User, groups config.Groups) (userOrgsRoleMap, error) {
	// Mapping of role per org
	userOrgsRole := make(userOrgsRoleMap)
	var isGlobalAdmin bool

	for _, group := range groups {
		// resolve grafana global admin
		isGlobalAdmin = isGlobalAdmin || group.GrafanaAdmin

		for _, org := range group.Orgs {
			// Check if the users has a more permissive role and apply that instead
			if !isRoleAssignable(userOrgsRole[org.ID], RoleType(org.Role)) {
				continue
			}

			userOrgsRole[org.ID] = RoleType(org.Role)
		}
	}

	// only update global admin value if it's different to what the user already have
	if user.IsAdmin != isGlobalAdmin {
		err := c.UpdateUserPermissions(user.ID, isGlobalAdmin)
		if err != nil {
			return userOrgsRole, err
		}
	}

	return userOrgsRole, nil
}

func (c *Client) GetOrCreateUser(login, name, email string) (gapi.User, error) {
	// lookup the user globally first as if it is not present it would need to
	// be created
	user, err := c.LookupUser(login)
	if err != nil {
		return user, err
	}

	// if the Login field in user is empty, it means that the user wasn't found
	if user.Login == "" {
		user.Login = login
		user.Name = name
		user.Email = email

		uid, err := c.CreateUser(user)
		if err != nil {
			return gapi.User{}, err
		}

		user.ID = uid
	}

	return user, nil
}

func isRoleAssignable(currentRole RoleType, incomingRole RoleType) bool {
	// role hierarchy
	roleHierarchy := map[RoleType]int{
		ROLE_VIEWER: 0,
		ROLE_EDITOR: 1,
		ROLE_ADMIN:  2,
	}

	// If the incoming role is less than ( less privilege ) than the currently assigned role ( more privilege ), skip this mapping.
	if currentRole != "" && roleHierarchy[RoleType(incomingRole)] < roleHierarchy[currentRole] {
		return false
	}

	return true
}
