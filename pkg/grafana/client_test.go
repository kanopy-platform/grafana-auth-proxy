package grafana

import (
	"errors"
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

	client := NewMockClient(user, nil)

	foundUser, err := client.LookupUser(user.Login)
	assert.Nil(t, err)
	assert.Equal(t, user.Login, foundUser.Login)

	notFoundUser, err := client.LookupUser("")
	assert.Nil(t, err)
	assert.Equal(t, gapi.User{}, notFoundUser)
}

func TestCreateUser(t *testing.T) {
	user := newUser("foo", 1)

	client := NewMockClient(user, nil)

	uid, err := client.CreateUser(user)
	assert.Nil(t, err)
	assert.Equal(t, uid, user.ID)
}

func TestAddOrgUser(t *testing.T) {
	user := newUser("foo", 1)

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
	user := newUser("foo", 1)

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
	user := newUser("foo", 1)

	client := NewMockClient(user, userOrgsRoleMap{})

	m, ok := client.client.(*mockGAPIClient)
	if !ok {
		t.Fail()
	}

	m.On("UpdateUserPermissions", int64(1), true).Return(nil)

	err := client.UpdateUserPermissions(user.ID, true)
	assert.NoError(t, err)

	m.AssertExpectations(t)
}

func TestUpdateOrgUserAuthz(t *testing.T) {
	adminUser := newUser("foo", 1)
	adminUser.IsAdmin = true

	tests := []struct {
		user                gapi.User
		groups              config.GroupsMap
		expected            userOrgsRoleMap
		expectedAdmin       bool
		expectedUpdateCalls int
		fail                bool
	}{
		{
			groups: config.GroupsMap{
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
			groups: config.GroupsMap{
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
		// user is Admin and 1 group in N groups is a grafana admin
		{
			user: adminUser,
			groups: config.GroupsMap{
				"foo": {
					GrafanaAdmin: true,
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
			expected:            userOrgsRoleMap{1: "Admin"},
			expectedAdmin:       true,
			expectedUpdateCalls: 0,
		},
		// user is not Admin and 1 group in N groups is a grafana admin
		{
			user: newUser("foo", 1),
			groups: config.GroupsMap{
				"foo": {
					GrafanaAdmin: true,
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
			expected:            userOrgsRoleMap{1: "Admin"},
			expectedAdmin:       true,
			expectedUpdateCalls: 1,
		},
		// user is not Admin and N groups have grafana admin
		{
			user: newUser("foo", 1),
			groups: config.GroupsMap{
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
			expected:            userOrgsRoleMap{1: "Admin"},
			expectedAdmin:       false,
			expectedUpdateCalls: 0,
		},
		// user is Admin and no group is a grafana admin
		{
			groups: config.GroupsMap{
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
			expected:            userOrgsRoleMap{1: "Editor"},
			fail:                true,
			expectedAdmin:       true,
			expectedUpdateCalls: 1,
		},
	}

	for _, test := range tests {
		// the client is only used to update grafana admin permissions in this case
		// so it doesn't matter what's the current value of user or orgMap is
		client := NewMockClient(test.user, userOrgsRoleMap{})

		m, ok := client.client.(*mockGAPIClient)
		if !ok {
			t.Fail()
		}

		if test.fail {
			m.On("UpdateUserPermissions", int64(0), true).Return(errors.New("error updating user permissions"))
		} else {
			// this tests the portion of `user.IsAdmin != isGlobalAdmin`
			if test.expectedUpdateCalls > 0 {
				m.On("UpdateUserPermissions", int64(1), test.expectedAdmin).Return(nil)
			}
		}

		orgsRoleMap, err := client.UpdateOrgUserAuthz(test.user, test.groups)

		m.AssertExpectations(t)
		m.AssertNumberOfCalls(t, "UpdateUserPermissions", test.expectedUpdateCalls)

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
