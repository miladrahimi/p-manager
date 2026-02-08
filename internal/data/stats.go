package data

import "time"

// Stats is a struct that represents the general statistics.
type Stats struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage" validate:"min=0"`
	TotalUsageBytes   int64   `json:"total_usage_bytes" validate:"min=0"`
}

// NewStats creates a new stats instance.
func NewStats(totalUsageResetAt int64, totalUsage float64, totalUsageBytes int64) *Stats {
	return &Stats{
		TotalUsageResetAt: totalUsageResetAt,
		TotalUsage:        totalUsage,
		TotalUsageBytes:   totalUsageBytes,
	}
}

// DefaultStats returns stats with default values.
func DefaultStats() *Stats {
	return NewStats(time.Now().UnixMilli(), 0, 0)
}
