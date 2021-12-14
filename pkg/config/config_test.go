package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		got  GroupsMap
		want GroupsMap
	}{
		{ // from empty config to loaded config
			got: GroupsMap{},
			want: GroupsMap{
				"foo": {
					Orgs: []Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
			},
		},
		{ // insert new group
			got: GroupsMap{
				"foo": {
					Orgs: []Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
			},
			want: GroupsMap{
				"foo": {
					Orgs: []Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
				"bar": {
					Orgs: []Org{
						{
							ID:   2,
							Role: "Editor",
						},
					},
				},
			},
		},
		{ // remove group
			got: GroupsMap{
				"foo": {
					Orgs: []Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
				"bar": {
					Orgs: []Org{
						{
							ID:   2,
							Role: "Editor",
						},
					},
				},
			},
			want: GroupsMap{
				"foo": {
					Orgs: []Org{
						{
							ID:   1,
							Role: "Editor",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		configGot := NewFromGroupsMap(test.got)

		fileWant := NewTestConfigFileFromGroups(t, test.want)
		defer os.Remove(fileWant)

		err := configGot.Load(fileWant)
		assert.NoError(t, err)

		assert.Equal(t, test.want, configGot.groups)
	}
}

func TestValidUserGroups(t *testing.T) {

	userGroups := []string{"foo", "bar", "baz"}

	got := GroupsMap{
		"foo": {
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
		"foo": {
			Orgs: []Org{
				{
					ID:   1,
					Role: "Editor",
				},
			},
		},
	}

	config := NewFromGroupsMap(got)

	validGroups := config.ValidUserGroups(userGroups)
	assert.Equal(t, want, validGroups)
}
