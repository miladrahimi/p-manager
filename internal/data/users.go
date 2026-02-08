package data

// User is a struct that represents a user in the application.
type User struct {
	Id                  int     `json:"id"`
	Identity            string  `json:"identity" validate:"required"`
	Name                string  `json:"name" validate:"required,min=1,max=64"`
	Quota               float64 `json:"quota" validate:"min=0"`
	Usage               float64 `json:"usage" validate:"min=0"`
	UsageBytes          int64   `json:"usage_bytes" validate:"min=0"`
	UsageResetAt        int64   `json:"usage_reset_at"`
	Enabled             bool    `json:"enabled"`
	ShadowsocksPassword string  `json:"shadowsocks_password" validate:"required,min=1,max=64"`
	ShadowsocksMethod   string  `json:"shadowsocks_method" validate:"required"`
	CreatedAt           int64   `json:"created_at"`
}

// NewUser creates a new user instance.
func NewUser(
	id int,
	identity string,
	name string,
	quota float64,
	usage float64,
	usageBytes int64,
	usageResetAt int64,
	enabled bool,
	shadowsocksPassword string,
	shadowsocksMethod string,
	createdAt int64,
) *User {
	return &User{
		Id:                  id,
		Identity:            identity,
		Name:                name,
		Quota:               quota,
		Usage:               usage,
		UsageBytes:          usageBytes,
		UsageResetAt:        usageResetAt,
		Enabled:             enabled,
		ShadowsocksPassword: shadowsocksPassword,
		ShadowsocksMethod:   shadowsocksMethod,
		CreatedAt:           createdAt,
	}
}
