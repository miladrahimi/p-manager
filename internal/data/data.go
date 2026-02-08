package data

import (
	"github.com/labstack/gommon/random"
	"github.com/miladrahimi/p-manager/pkg/util"
)

// Data is the database data (schema).
type Data struct {
	Settings *Settings `json:"settings"`
	Stats    *Stats    `json:"stats"`
	Users    []*User   `json:"users"`
	Nodes    []*Node   `json:"nodes"`
}

// New creates a new instance of Data.
func New(settings *Settings, stats *Stats, users []*User, nodes []*Node) *Data {
	return &Data{
		Settings: settings,
		Stats:    stats,
		Users:    users,
		Nodes:    nodes,
	}
}

// Default returns a new data with default values.
func Default() *Data {
	return New(
		DefaultSettings(),
		DefaultStats(),
		[]*User{},
		[]*Node{},
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

// GenerateUserIdentity generates a unique identity for a new user.
func (s *Data) GenerateUserIdentity() string {
	return util.UUID()
}

// GenerateUserPassword generates a unique password for a new user.
func (s *Data) GenerateUserPassword() string {
	for {
		r := random.String(16)
		isUnique := true
		for _, user := range s.Users {
			if user.ShadowsocksPassword == r {
				isUnique = false
				break
			}
		}
		if isUnique {
			return r
		}
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
