package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidUserGroups(t *testing.T) {

	userGroups := []string{"one", "two", "three"}

	var configString = `
groups:
  two:
    orgs:
      - id: 1
        role: Editor
  new:
    orgs:
      - id: 3
        role: Admin
`

	want := GroupsMap{
		"two": {
			Orgs: []Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
	}

	config := NewTestConfigFromString(t, configString)

	validGroups := config.ValidUserGroups(userGroups)
	assert.Equal(t, want, validGroups)
}
