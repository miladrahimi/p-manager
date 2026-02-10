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

// FindUserById finds a user by its (primary) ID.
func (s *Data) FindUserById(id string) *User {
	for _, u := range s.Users {
		if u.Id == id {
			return u
		}
	}
	return nil
}
