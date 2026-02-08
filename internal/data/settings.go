package data

const defaultAdminPassword = "password"
const defaultHost = "127.0.0.1"
const defaultTrafficRatio = 1

// Settings is a struct that represents the settings of the application.
type Settings struct {
	AdminPassword string  `json:"admin_password" validate:"required,min=8,max=32"`
	Host          string  `json:"host" validate:"required,max=128"`
	SsReversePort int     `json:"ss_reverse_port" validate:"min=0,max=65535"`
	SsRelayPort   int     `json:"ss_relay_port" validate:"min=0,max=65535"`
	SsDirectPort  int     `json:"ss_direct_port" validate:"min=0,max=65535"`
	SsRemotePort  int     `json:"ss_remote_port" validate:"min=0,max=65535"`
	TrafficRatio  float64 `json:"traffic_ratio" validate:"min=1,max=1024"`
	SingetServer  string  `json:"singet_server" validate:"omitempty,url"`
	ResetPolicy   string  `json:"reset_policy" validate:"omitempty,oneof=monthly"`
}

// NewSettings creates a new settings instance.
func NewSettings(
	adminPassword string,
	host string,
	ssReversePort int,
	ssRelayPort int,
	ssDirectPort int,
	ssRemotePort int,
	trafficRatio float64,
	singetServer string,
	resetPolicy string,
) *Settings {
	return &Settings{
		AdminPassword: adminPassword,
		Host:          host,
		SsReversePort: ssReversePort,
		SsRelayPort:   ssRelayPort,
		SsDirectPort:  ssDirectPort,
		SsRemotePort:  ssRemotePort,
		TrafficRatio:  trafficRatio,
		SingetServer:  singetServer,
		ResetPolicy:   resetPolicy,
	}
}

// DefaultSettings returns the settings with default values.
func DefaultSettings() *Settings {
	return NewSettings(
		defaultAdminPassword,
		defaultHost,
		0,
		0,
		0,
		0,
		defaultTrafficRatio,
		"",
		"",
	)
}
