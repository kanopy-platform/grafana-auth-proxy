package grafana

import (
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
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

	notFoundUser, err := client.LookupUser("")
	assert.Nil(t, err)
	assert.Equal(t, gapi.User{}, notFoundUser)
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

	orgRoleMap := userOrgsRoleMap{
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

	orgRoleMap := userOrgsRoleMap{
		1: models.ROLE_EDITOR,
	}

	client := NewMockClient(user, orgRoleMap)

	// this should always succeed except for errors when calling the rest api
	err := client.UpsertOrgUser(1, user, "Editor")
	assert.Nil(t, err)

	// upsert will return an error when the orgID is invalid for example
	err = client.UpsertOrgUser(0, user, "Admin")
	assert.NotNil(t, err)

	// if user doesn't exists then Upsert will return an error on update user path
	err = client.UpsertOrgUser(1, gapi.User{}, "Viewer")
	assert.NotNil(t, err)
}

// This is a silly test as the mock always returns nil but it's here for completeness
func TestUpdateUserPermissions(t *testing.T) {
	user := setupUser()

	client := NewMockClient(user, userOrgsRoleMap{})

	err := client.UpdateUserPermissions(user.ID, true)
	assert.NoError(t, err)
}

func TestUpdateUserGrafanaAdmin(t *testing.T) {
	user := setupUser()

	client := NewMockClient(user, userOrgsRoleMap{})

	err := client.updateUserGrafanaAdmin(user, true)
	assert.NoError(t, err)

	err = client.updateUserGrafanaAdmin(user, false)
	assert.NoError(t, err)
}

func TestUpdateOrgUserAuthz(t *testing.T) {
	user := setupUser()

	// the client is only used to update grafana admin permissions in this case
	// so it doesn't matter what's the current value of user or orgMap is
	client := NewMockClient(user, userOrgsRoleMap{})

	tests := []struct {
		groups   config.Groups
		expected userOrgsRoleMap
		fail     bool
	}{
		{
			groups: config.Groups{
				"foo": {
					Orgs: []config.Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
				"bar": {
					Orgs: []config.Org{
						{
							ID:   1,
							Role: "Admin",
						},
					},
				},
			},
			expected: userOrgsRoleMap{1: "Admin"},
		},
		{
			groups: config.Groups{
				"foo": {
					Orgs: []config.Org{
						{
							ID:   1,
							Role: "Admin",
						},
					},
				},
				"bar": {
					Orgs: []config.Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
			},
			expected: userOrgsRoleMap{1: "Admin"},
		},
		{
			groups: config.Groups{
				"foo": {
					GrafanaAdmin: true,
					Orgs: []config.Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
			},
			expected: userOrgsRoleMap{},
			fail:     true,
		},
	}

	for _, test := range tests {
		var withError bool

		if test.fail {
			// setting user.ID to 0 forces an error in UpdateUserPermissions that is used
			// by the grafana admin update API
			user.ID = 0
			user.IsAdmin = false
			withError = true
		}

		orgsRoleMap, err := client.UpdateOrgUserAuthz(user, test.groups)
		assert.Equal(t, test.expected, orgsRoleMap)
		if withError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestGetOrCreateUser(t *testing.T) {

	// Existing user
	user := setupUser()

	client := NewMockClient(user, userOrgsRoleMap{})

	orgUser, err := client.GetOrCreateUser("foo")
	assert.NoError(t, err)
	assert.Equal(t, user, orgUser)

	// New user
	newUser, err := client.GetOrCreateUser("new")
	assert.NoError(t, err)
	// for convenience the CreateUser mock returns the same ID as the user.ID
	// passed in NewMockClient
	assert.Equal(t, int64(1), newUser.ID)
}

func TestIsRoleAssignable(t *testing.T) {
	// table test to  validate isRoleAssignable(currentRole, incomingRole)
	assert.True(t, isRoleAssignable("", models.ROLE_VIEWER))
	assert.True(t, isRoleAssignable(models.ROLE_VIEWER, models.ROLE_EDITOR))
	assert.True(t, isRoleAssignable(models.ROLE_VIEWER, models.ROLE_ADMIN))
	assert.True(t, isRoleAssignable(models.ROLE_EDITOR, models.ROLE_ADMIN))
	assert.False(t, isRoleAssignable(models.ROLE_ADMIN, models.ROLE_EDITOR))
	assert.False(t, isRoleAssignable(models.ROLE_ADMIN, models.ROLE_VIEWER))
	assert.False(t, isRoleAssignable(models.ROLE_EDITOR, models.ROLE_VIEWER))
	assert.True(t, isRoleAssignable(models.ROLE_VIEWER, models.ROLE_VIEWER))

	roles := map[int64]models.RoleType{}
	assert.True(t, isRoleAssignable(roles[0], models.ROLE_VIEWER))

}
