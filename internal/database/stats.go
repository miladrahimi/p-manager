package database

type Stats struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage" validate:"min=0"`
	TotalUsageBytes   int64   `json:"total_usage_bytes" validate:"min=0"`
}
