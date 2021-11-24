package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// UserGroupsInConfig matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func TestValidUserGroups(t *testing.T) {

	userGroups := []string{"one", "two", "three"}

	expectedGroups := Groups{
		"two": {
			GrafanaAdmin: false,
			Orgs: []Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
	}

	groups := map[string]Group{
		"two": {
			Orgs: []Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
		"new": {
			Orgs: []Org{
				{
					ID:   3,
					Role: "Admin",
				},
			},
		},
	}

	validGroups := ValidUserGroups(userGroups, groups)
	assert.Equal(t, expectedGroups, validGroups)
}
