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
		return nil, err
	}

	// gapi returns an empty struct
	if user.Login == "" {
		return nil, err
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

// UpsertOrgUser adds a user to an Organization, creating it, if it doesn't exists
func (c *Client) UpsertOrgUser(OrgId int64, user gapi.User, role string) error {
	foundUser, err := c.LookupUser(user.Login)
	if err != nil {
		if !strings.Contains(err.Error(), "User not found") {
			return err
		}
	}

	if foundUser == nil {
		log.Infof("no user with login %s found, creating new one", user.Login)

		uid, err := c.CreateUser(user)
		if err != nil {
			log.Error(err)
			return err
		}

		log.Infof("new user %s created with id %d", user.Login, uid)
	}

	err = c.AddOrgUser(OrgId, user.Login, role)
	if err != nil {
		if !strings.Contains(err.Error(), "User is already member of this organization") {
			log.Error(err)
			return err
		}
	}

	return nil
}
