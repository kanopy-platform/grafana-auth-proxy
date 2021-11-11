package grafana

import (
	gapi "github.com/grafana/grafana-api-golang-client"
)

type mockUserInOrg struct {
	member        bool
	responseError error
}

// MockGAPIClient mimicks the behaviour of required gapi calls
type mockGAPIClient struct {
	user      gapi.User
	userInOrg *mockUserInOrg
}

func (c *mockGAPIClient) UserByEmail(login string) (gapi.User, error) {
	return c.user, nil
}

func (c *mockGAPIClient) CreateUser(user gapi.User) (int64, error) {
	return c.user.ID, nil
}

func (c *mockGAPIClient) AddOrgUser(OrgID int64, login string, role string) error {
	if c.userInOrg.member {
		return c.userInOrg.responseError
	}

	return nil
}

// MockClient returns a Client using a mocked GAPIClient underneat
func NewMockClient(user gapi.User, userInOrg *mockUserInOrg) *Client {
	return &Client{
		client: &mockGAPIClient{
			user:      user,
			userInOrg: userInOrg,
		},
	}
}
