package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// UserGroupsInConfig matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func TestValidUserGroups(t *testing.T) {

	userGroups := []string{"one", "two", "three"}

	groups := map[string]Group{
		"two": {
			Orgs: []Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
	}

	validGroups := ValidUserGroups(userGroups, groups)
	assert.Contains(t, validGroups, "two")
	assert.NotContains(t, validGroups, "one")
}
