package grafana

import (
	"fmt"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/grafana/pkg/models"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/stretchr/testify/assert"
)

func newUser(login string, id int64) gapi.User {
	return gapi.User{
		ID:    id,
		Email: fmt.Sprintf("%s@example.com", login),
		Login: login,
	}
}

func TestLookupUser(t *testing.T) {
	user := newUser("foo", 1)

	client := NewMockClient(&user, nil)

	foundUser, err := client.LookupUser(user.Login)
	assert.Nil(t, err)
	assert.Equal(t, user.Login, foundUser.Login)

	notFoundUser, err := client.LookupUser("")
	assert.Nil(t, err)
	assert.Equal(t, gapi.User{}, notFoundUser)
}

func TestCreateUser(t *testing.T) {
	user := newUser("foo", 1)

	client := NewMockClient(&user, nil)

	uid, err := client.CreateUser(user)
	assert.Nil(t, err)
	assert.Equal(t, uid, user.ID)
}

func TestAddOrgUser(t *testing.T) {
	user := newUser("foo", 1)

	orgRoleMap := userOrgsRoleMap{
		1: models.ROLE_EDITOR,
	}
	client := NewMockClient(&user, orgRoleMap)

	// test adding to new org
	err := client.AddOrgUser(2, "foo", "Editor")
	assert.NoError(t, err)

	// test already a member
	err = client.AddOrgUser(1, "foo", "Editor")
	assert.Contains(t, err.Error(), "User is already member")
}

func TestUpsertOrgUser(t *testing.T) {
	user := newUser("foo", 1)

	orgRoleMap := userOrgsRoleMap{
		1: models.ROLE_EDITOR,
	}

	client := NewMockClient(&user, orgRoleMap)

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
	user := newUser("foo", 1)

	client := NewMockClient(&user, userOrgsRoleMap{})

	err := client.UpdateUserPermissions(user.ID, true)
	assert.NoError(t, err)
}

func TestUpdateOrgUserAuthz(t *testing.T) {
	tests := []struct {
		user     gapi.User
		groups   config.Groups
		expected userOrgsRoleMap
		fail     bool
	}{
		{
			user: newUser("foo", 1),
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
			user: newUser("foo", 1),
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
		// Using user id 0 forces an error in UpdateUserPermissions
		// GrafanaAdmin is set to true to make it different than users's default
		// isAdmin value
		{
			user: newUser("foo", 0),
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
			expected: userOrgsRoleMap{1: "Editor"},
			fail:     true,
		},
	}

	for _, test := range tests {
		// the client is only used to update grafana admin permissions in this case
		// so it doesn't matter what's the current value of user or orgMap is
		client := NewMockClient(test.user, userOrgsRoleMap{})

		orgsRoleMap, err := client.UpdateOrgUserAuthz(test.user, test.groups)

		if !test.fail {
			assert.Equal(t, test.expected, orgsRoleMap)
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestGetOrCreateUser(t *testing.T) {
	// Existing user
	user := newUser("foo", 1)

	client := NewMockClient(&user, userOrgsRoleMap{})

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
