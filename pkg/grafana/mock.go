package grafana

import (
	"errors"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
)

// MockGAPIClient mimicks the behaviour of required gapi calls
type mockGAPIClient struct {
	user       gapi.User
	orgRoleMap map[int64]models.RoleType
}

func (c *mockGAPIClient) UserByEmail(login string) (gapi.User, error) {
	if c.user.Login == login {
		return c.user, nil
	}

	return gapi.User{}, errors.New(`body: "User not found"`)
}

func (c *mockGAPIClient) CreateUser(user gapi.User) (int64, error) {
	return user.ID, nil
}

func (c *mockGAPIClient) AddOrgUser(OrgID int64, login string, role string) error {

	if _, ok := c.orgRoleMap[OrgID]; ok {
		return errors.New(`status: 409, body: "User is already member of this organization"`)
	}

	return nil
}

func (c *mockGAPIClient) UpdateOrgUser(orgID, userID int64, role string) error {
	return nil
}

// MockClient returns a Client using a mocked GAPIClient underneat
func NewMockClient(user gapi.User, orgRoleMap map[int64]models.RoleType) *Client {
	return &Client{
		client: &mockGAPIClient{
			user:       user,
			orgRoleMap: orgRoleMap,
		},
	}
}
