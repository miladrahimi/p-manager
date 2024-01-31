package database

type Settings struct {
	AdminPassword string  `json:"admin_password" validate:"required,min=8,max=32"`
	Host          string  `json:"host" validate:"required,max=128"`
	SsrPort       int     `json:"ssr_port" validate:"required,min=1,max=65536"`
	SsdPort       int     `json:"ssd_port" validate:"required,min=1,max=65536"`
	TrafficRatio  float64 `json:"traffic_ratio" validate:"required,min=1,max=1024"`
}
