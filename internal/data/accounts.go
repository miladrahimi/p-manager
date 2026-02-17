package data

// Account is a struct that represents an account in the application.
type Account struct {
	Id           string  `json:"id" validate:"required,uuid"`
	ProxyId      string  `json:"proxy_id" validate:"omitempty,uuid"`
	Name         string  `json:"name" validate:"required,min=1,max=64"`
	Quota        float64 `json:"quota" validate:"min=0"`
	Usage        float64 `json:"usage" validate:"min=0"`
	UsageBytes   int64   `json:"usage_bytes" validate:"min=0"`
	UsageResetAt int64   `json:"usage_reset_at"`
	Enabled      bool    `json:"enabled"`
	CreatedAt    int64   `json:"created_at"`
}

// NewAccount creates a new account instance.
func NewAccount(
	id string,
	proxyId string,
	name string,
	quota float64,
	usage float64,
	usageBytes int64,
	usageResetAt int64,
	enabled bool,
	createdAt int64,
) *Account {
	return &Account{
		Id:           id,
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
