package database

type Settings struct {
	AdminPassword   string  `json:"admin_password" validate:"required,min=8,max=32"`
	ShadowsocksHost string  `json:"shadowsocks_host" validate:"required,max=128"`
	ShadowsocksPort int     `json:"shadowsocks_port" validate:"required,min=1,max=65536"`
	HttpsAddress    string  `json:"https_address" validate:"max=128"`
	HttpAddress     string  `json:"http_address" validate:"required,max=128"`
	TrafficRatio    float64 `json:"traffic_ratio" validate:"required,min=1,max=1024"`
}
