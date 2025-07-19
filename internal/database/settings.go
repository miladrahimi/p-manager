package database

type Settings struct {
	AdminPassword string  `json:"admin_password" validate:"required,min=8,max=32"`
	Host          string  `json:"host" validate:"required,max=128"`
	SsReversePort int     `json:"ss_reverse_port" validate:"min=0,max=65536"`
	SsRelayPort   int     `json:"ss_relay_port" validate:"min=0,max=65536"`
	SsDirectPort  int     `json:"ss_direct_port" validate:"min=0,max=65536"`
	SsRemotePort  int     `json:"ss_remote_port" validate:"min=0,max=65536"`
	TrafficRatio  float64 `json:"traffic_ratio" validate:"min=1,max=1024"`
	SingetServer  string  `json:"singet_server" validate:"omitempty,url"`
	ResetPolicy   string  `json:"reset_policy" validate:"omitempty,oneof=monthly"`
}
