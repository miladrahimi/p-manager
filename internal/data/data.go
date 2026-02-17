package data

// Data is the database data (schema).
type Data struct {
	Accounts     []*Account    `json:"accounts"`
	Nodes        []*Node       `json:"nodes"`
	Stats        *Stats        `json:"stats"`
	MainSettings *Settings     `json:"main_settings"`
	XraySettings *XraySettings `json:"xray_settings"`
}

// New creates a new instance of Data.
func New(accounts []*Account, nodes []*Node, stats *Stats, mainSettings *Settings, xraySettings *XraySettings) *Data {
	return &Data{
		Accounts:     accounts,
		Nodes:        nodes,
		Stats:        stats,
		MainSettings: mainSettings,
		XraySettings: xraySettings,
	}
}

// Default returns a new data with default values.
func Default() *Data {
	return New(
		[]*Account{},
		[]*Node{},
		DefaultStats(),
		DefaultSettings(),
		DefaultXray(),
	)
}

// CountActiveAccounts counts the number of active accounts.
func (s *Data) CountActiveAccounts() int {
	activeAccountsCount := len(s.Accounts)
	for _, u := range s.Accounts {
		if !u.Enabled {
			activeAccountsCount--
		}
	}
	return activeAccountsCount
}

// FindAccountById finds an account by its (primary) ID.
func (s *Data) FindAccountById(id string) *Account {
	for _, u := range s.Accounts {
		if u.Id == id {
			return u
		}
	}
	return nil
}

// FindAccountByProxyId finds an account by its proxy ID.
func (s *Data) FindAccountByProxyId(proxyId string) *Account {
	for _, u := range s.Accounts {
		if u.ProxyId == proxyId {
			return u
		}
	}
	return nil
}
