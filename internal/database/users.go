package database

type User struct {
	Id                  int     `json:"id"`
	Identity            string  `json:"identity" validate:"required"`
	Name                string  `json:"name" validate:"required,min=1,max=64"`
	Quota               int     `json:"quota" validate:"min=0"`
	Used                float64 `json:"used" validate:"min=0"`
	UsedBytes           int64   `json:"used_bytes" validate:"min=0"`
	Enabled             bool    `json:"enabled"`
	CreatedAt           int64   `json:"created_at"`
	ShadowsocksPassword string  `json:"shadowsocks_password" validate:"required,min=1,max=64"`
	ShadowsocksMethod   string  `json:"shadowsocks_method" validate:"required"`
}
