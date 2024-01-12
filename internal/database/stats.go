package database

type Stats struct {
	UpdatedAt int64 `json:"updated_at"`
	Inbound   int64 `json:"inbound"`
	Outbound  int64 `json:"outbound"`
	Freedom   int64 `json:"freedom"`
}
