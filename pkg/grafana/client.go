package grafana

import (
	"net/url"
	"strings"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/k8s-auth-portal/pkg/random"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	client GAPIClient
}

type GAPIClient interface {
	UserByEmail(email string) (user gapi.User, err error)
	CreateUser(user gapi.User) (int64, error)
	AddOrgUser(orgID int64, user, role string) error
	UpdateOrgUser(orgID, userID int64, role string) error
}

func NewClient(baseURL *url.URL, cfg gapi.Config) (*Client, error) {
	newClient := &Client{}

	client, err := gapi.New(baseURL.String(), cfg)
	if err != nil {
		return nil, err
	}

	newClient.client = client

	return newClient, nil
}

// LookupUser search for a user by Login or Email and returns it
func (c *Client) LookupUser(loginOrEmail string) (*gapi.User, error) {
	user, err := c.client.UserByEmail(loginOrEmail)
	if err != nil {
		if !strings.Contains(err.Error(), "User not found") {
			return nil, err
		}
	}

	// gapi returns an empty struct
	if user.Login == "" {
		return nil, nil
	}

	return &user, nil
}

// CreateUser adds a new global user to Grafana
func (c *Client) CreateUser(user gapi.User) (int64, error) {
	var uid int64

	// The Grafana API requires a password for user creation
	if user.Password == "" {
		// Generate new random password
		sstring, err := random.SecureString(12)
		if err != nil {
			log.Errorf("error generating random password: %v", err)

			return uid, err
		}

		user.Password = sstring
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
			log.Error(err)
			return err
		} else {
			isOrgMember = true
		}
	}

	if isOrgMember {
		// Update user if it's a member in case roles changed in config
		log.Infof("updating user, %s", user.Login)
		log.Debugf("user object, %v", user)
		err = c.client.UpdateOrgUser(orgID, user.ID, role)
		if err != nil {
			return err
		}
	}

	return nil
}
