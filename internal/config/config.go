package config

import "github.com/spf13/viper"

type Group struct {
	Orgs []Org `json:"orgs"`
}

type Groups map[string]Group

type Org struct {
	OrgId int64  `json:"orgId"`
	Role  string `json:"role"`
}

func ParseGroups() (Groups, error) {
	groups := Groups{}
	if err := viper.UnmarshalKey("groups", &groups); err != nil {
		return groups, err
	}

	return groups, nil
}

// UserGroupsInConfig matches the user groups (from claims) that are
// present in config
func UserGroupsInConfig(userGroups []string, groups Groups) []string {
	var matchedGroups []string
	for _, userGroup := range userGroups {
		if _, ok := groups[userGroup]; ok {
			matchedGroups = append(matchedGroups, userGroup)
		}
	}

	return matchedGroups
}
