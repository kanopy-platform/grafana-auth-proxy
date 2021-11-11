package grafana

import (
	"errors"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/stretchr/testify/assert"
)

func setupUser() gapi.User {
	return gapi.User{
		ID:    1,
		Email: "foo@example.com",
		Login: "foo",
	}
}

func TestLookupUser(t *testing.T) {
	user := setupUser()

	client := NewMockClient(user, nil)

	_, err := client.LookupUser(user.Login)
	assert.Nil(t, err)
}

func TestCreateUser(t *testing.T) {
	user := setupUser()

	client := NewMockClient(user, nil)

	uid, err := client.CreateUser(user)
	assert.Nil(t, err)
	assert.Equal(t, uid, user.ID)
}

func TestAddOrgUser(t *testing.T) {
	user := setupUser()

	client := NewMockClient(user, &mockUserInOrg{member: false, responseError: nil})

	err := client.AddOrgUser(1, "foo", "Editor")
	assert.NoError(t, err)
}

func TestUpsertOrgUser(t *testing.T) {

	user := gapi.User{
		ID:    1,
		Email: "foo@example.com",
		Login: "foo",
	}

	client := NewMockClient(
		user,
		&mockUserInOrg{
			member:        true,
			responseError: errors.New(`status: 409, body: "User is already member of this organization"`),
		},
	)

	err := client.UpsertOrgUser(3, user, "Editor")
	assert.Nil(t, err)
}
