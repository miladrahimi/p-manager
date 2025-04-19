package database

type Stats struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage"`
}
