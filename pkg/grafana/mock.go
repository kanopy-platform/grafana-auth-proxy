package grafana

import (
	"errors"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
)

// MockGAPIClient mimicks the behaviour of required gapi calls
type mockGAPIClient struct {
	user       *gapi.User
	orgRoleMap userOrgsRoleMap
}

func (c *mockGAPIClient) UserByEmail(login string) (gapi.User, error) {
	if c.user.Login == login {
		return *c.user, nil
	}

	return gapi.User{}, errors.New(`body: "User not found"`)
}

func (c *mockGAPIClient) CreateUser(user gapi.User) (int64, error) {
	// for new user return the ID of the provided `user` in mockGAPIClient
	return c.user.ID, nil
}

func (c *mockGAPIClient) AddOrgUser(orgID int64, login string, role string) error {
	if _, ok := c.orgRoleMap[orgID]; ok {
		return errors.New(`status: 409, body: "User is already member of this organization"`)
	}

	// force an error when orgID is 0
	// Grafana starts orgIDs from 1 so this is a fair assumption
	if orgID == 0 {
		return errors.New("orgID does not exists")
	}

	return nil
}

func (c *mockGAPIClient) UpdateOrgUser(orgID, userID int64, role string) error {
	if userID == 0 {
		return errors.New("user has no id")
	}

	return nil
}

func (c *mockGAPIClient) UpdateUserPermissions(id int64, isAdmin bool) error {
	if id == 0 {
		return errors.New("error updating user permissions")
	}

	c.user.IsAdmin = isAdmin

	return nil
}

// MockClient returns a Client using a mocked GAPIClient underneat
func NewMockClient(user *gapi.User, orgRoleMap map[int64]models.RoleType) *Client {
	return &Client{
		client: &mockGAPIClient{
			user:       user,
			orgRoleMap: orgRoleMap,
		},
	}
}
