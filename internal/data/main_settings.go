package data

const defaultAdminPassword = "password"
const defaultHost = "127.0.0.1"
const defaultTrafficRatio = 1
const defaultSubscriptionEnabled = false

// Settings is a struct that represents the settings of the application.
type Settings struct {
	AdminPassword string  `json:"admin_password" validate:"required,min=8,max=32"`
	Host          string  `json:"host" validate:"required,max=128"`
	TrafficRatio  float64 `json:"traffic_ratio" validate:"min=1,max=1024"`
	SingetServer  string  `json:"singet_server" validate:"omitempty,url"`
	ResetPolicy   string  `json:"reset_policy" validate:"omitempty,oneof=monthly"`
	// SubscriptionEnabled shows the subscription link (all servers in one URL) on the account page.
	// It only affects the account page UI: when off, accounts see just the individual server links,
	// while the subscription endpoint itself keeps working.
	SubscriptionEnabled bool `json:"subscription_enabled"`
	// HetznerToken is a Hetzner Cloud project API token (read & write). When set,
	// the nodes page can create a server on Hetzner and install P-Node on it.
	HetznerToken string `json:"hetzner_token" validate:"max=128"`
}

// NewSettings creates a new settings instance.
func NewSettings(
	adminPassword string,
	host string,
	trafficRatio float64,
	singetServer string,
	resetPolicy string,
	subscriptionEnabled bool,
	hetznerToken string,
) *Settings {
	return &Settings{
		AdminPassword:       adminPassword,
		Host:                host,
		TrafficRatio:        trafficRatio,
		SingetServer:        singetServer,
		ResetPolicy:         resetPolicy,
		SubscriptionEnabled: subscriptionEnabled,
		HetznerToken:        hetznerToken,
	}
}

// DefaultSettings returns the settings with default values.
func DefaultSettings() *Settings {
	return NewSettings(
		defaultAdminPassword,
		defaultHost,
		defaultTrafficRatio,
		"",
		"",
		defaultSubscriptionEnabled,
		"",
	)
}
