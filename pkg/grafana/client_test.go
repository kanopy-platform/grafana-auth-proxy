package grafana

import (
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
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

	foundUser, err := client.LookupUser(user.Login)
	assert.Nil(t, err)
	assert.Equal(t, user.Login, foundUser.Login)
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

	orgRoleMap := map[int64]models.RoleType{
		1: models.ROLE_EDITOR,
	}
	client := NewMockClient(user, orgRoleMap)

	// test adding to new org
	err := client.AddOrgUser(2, "foo", "Editor")
	assert.NoError(t, err)

	// test already a member
	err = client.AddOrgUser(1, "foo", "Editor")
	assert.Contains(t, err.Error(), "User is already member")
}

func TestUpsertOrgUser(t *testing.T) {
	user := setupUser()

	orgRoleMap := map[int64]models.RoleType{
		1: models.ROLE_EDITOR,
	}

	client := NewMockClient(user, orgRoleMap)

	// this should always succedd except for error when calling the rest api
	err := client.UpsertOrgUser(1, user, "Editor")
	assert.Nil(t, err)
}

func TestIsRoleAssignable(t *testing.T) {
	// table test to  validate isRoleAssignable(currentRole, incomingRole)
	assert.True(t, IsRoleAssignable("", models.ROLE_VIEWER))
	assert.True(t, IsRoleAssignable(models.ROLE_VIEWER, models.ROLE_EDITOR))
	assert.True(t, IsRoleAssignable(models.ROLE_VIEWER, models.ROLE_ADMIN))
	assert.True(t, IsRoleAssignable(models.ROLE_EDITOR, models.ROLE_ADMIN))
	assert.False(t, IsRoleAssignable(models.ROLE_ADMIN, models.ROLE_EDITOR))
	assert.False(t, IsRoleAssignable(models.ROLE_ADMIN, models.ROLE_VIEWER))
	assert.False(t, IsRoleAssignable(models.ROLE_EDITOR, models.ROLE_VIEWER))
	assert.True(t, IsRoleAssignable(models.ROLE_VIEWER, models.ROLE_VIEWER))

	roles := map[int64]models.RoleType{}
	assert.True(t, IsRoleAssignable(roles[0], models.ROLE_VIEWER))

}
