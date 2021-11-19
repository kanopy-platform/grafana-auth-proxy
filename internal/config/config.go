package config

type Group struct {
	GrafanaAdmin bool  `json:"grafanaAdmin,omitempty"`
	Orgs         []Org `json:"orgs"`
}

type Groups map[string]Group

type Org struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

// UserGroupsInConfig matches the user groups (from claims) that are
// present in config and returns a filtered set of Groups
func ValidUserGroups(userGroups []string, groups Groups) Groups {
	finalGroups := make(map[string]Group)

	for _, userGroup := range userGroups {
		if _, ok := groups[userGroup]; ok {
			finalGroups[userGroup] = groups[userGroup]
		}
	}

	return finalGroups
}
