package data

// User is a struct that represents a user in the application.
type User struct {
	Id           string  `json:"id" validate:"required,uuid"`
	VlessId      string  `json:"vless_id" validate:"omitempty,uuid"`
	ProxyId      string  `json:"proxy_id" validate:"omitempty,uuid"`
	Name         string  `json:"name" validate:"required,min=1,max=64"`
	Quota        float64 `json:"quota" validate:"min=0"`
	Usage        float64 `json:"usage" validate:"min=0"`
	UsageBytes   int64   `json:"usage_bytes" validate:"min=0"`
	UsageResetAt int64   `json:"usage_reset_at"`
	Enabled      bool    `json:"enabled"`
	CreatedAt    int64   `json:"created_at"`
}

// NewUser creates a new user instance.
func NewUser(
	id string,
	proxyId string,
	name string,
	quota float64,
	usage float64,
	usageBytes int64,
	usageResetAt int64,
	enabled bool,
	createdAt int64,
) *User {
	return &User{
		Id:           id,
		VlessId:      proxyId,
		ProxyId:      proxyId,
		Name:         name,
		Quota:        quota,
		Usage:        usage,
		UsageBytes:   usageBytes,
		UsageResetAt: usageResetAt,
		Enabled:      enabled,
		CreatedAt:    createdAt,
	}
}
