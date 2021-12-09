package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// UserGroupsInConfig matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func TestValidUserGroups(t *testing.T) {

	userGroups := []string{"one", "two", "three"}

	got := GroupsMap{
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

	// tmpFile, err := ioutil.TempFile(os.TempDir(), "grafana-proxy-")
	// assert.NoError(t, err)
	// defer os.Remove(tmpFile.Name())

	// data, err := yaml.Marshal(&got)
	// assert.NoError(t, err)

	// fmt.Println(string(data))

	// _, err = tmpFile.Write(data)
	// assert.NoError(t, err)
	// err = tmpFile.Close()
	// assert.NoError(t, err)

	// config := New()
	// err = config.Load(tmpFile.Name())
	// assert.NoError(t, err)

	validGroups := ValidUserGroups(userGroups, got)
	assert.Equal(t, want, validGroups)
}
