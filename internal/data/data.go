package data

// Data is the database data (schema).
type Data struct {
	Users        []*User       `json:"users"`
	Nodes        []*Node       `json:"nodes"`
	Stats        *Stats        `json:"stats"`
	MainSettings *Settings     `json:"main_settings"`
	XraySettings *XraySettings `json:"xray_settings"`
}

// New creates a new instance of Data.
func New(users []*User, nodes []*Node, stats *Stats, mainSettings *Settings, xraySettings *XraySettings) *Data {
	return &Data{
		Users:        users,
		Nodes:        nodes,
		Stats:        stats,
		MainSettings: mainSettings,
		XraySettings: xraySettings,
	}
}

// Default returns a new data with default values.
func Default() *Data {
	return New(
		[]*User{},
		[]*Node{},
		DefaultStats(),
		DefaultSettings(),
		DefaultXray(),
	)
}

// CountActiveUsers counts the number of active users.
func (s *Data) CountActiveUsers() int {
	activeUsersCount := len(s.Users)
	for _, u := range s.Users {
		if !u.Enabled {
			activeUsersCount--
		}
	}
	return activeUsersCount
}

// GenerateUserId generates a unique ID for a new user.
func (s *Data) GenerateUserId() int {
	if len(s.Users) > 0 {
		return s.Users[len(s.Users)-1].Id + 1
	} else {
		return 1
	}
}

// GenerateNodeId generates a unique ID for a new node.
func (s *Data) GenerateNodeId() int {
	if len(s.Nodes) > 0 {
		return s.Nodes[len(s.Nodes)-1].Id + 1
	} else {
		return 1
	}
}
