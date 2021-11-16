package config

type Group struct {
	Orgs []Org `json:"orgs"`
}

type Groups map[string]Group

type Org struct {
	ID           int64  `json:"id"`
	Role         string `json:"role"`
	GrafanaAdmin bool   `json:"grafanaAdmin"`
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
