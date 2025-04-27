package database

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
