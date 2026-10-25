package data

import "time"

// Stats is a struct that represents the general statistics.
type Stats struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage" validate:"min=0"`
	TotalUsageBytes   int64   `json:"total_usage_bytes" validate:"min=0"`
	// MethodUsageBytes is the traffic per proxy method, keyed by the client
	// inbound tag (direct-rr, remote-rr, remote-cxh, ...); it resets with the total.
	MethodUsageBytes map[string]int64 `json:"method_usage_bytes"`
}

// NewStats creates a new stats instance.
func NewStats(totalUsageResetAt int64, totalUsage float64, totalUsageBytes int64) *Stats {
	return &Stats{
		TotalUsageResetAt: totalUsageResetAt,
		TotalUsage:        totalUsage,
		TotalUsageBytes:   totalUsageBytes,
		MethodUsageBytes:  map[string]int64{},
	}
}

// AddMethodUsage adds bytes to a method's usage (the map may be missing in
// databases written before it existed).
func (s *Stats) AddMethodUsage(method string, bytes int64) {
	if s.MethodUsageBytes == nil {
		s.MethodUsageBytes = map[string]int64{}
	}
	s.MethodUsageBytes[method] += bytes
}

// DefaultStats returns stats with default values.
func DefaultStats() *Stats {
	return NewStats(time.Now().UnixMilli(), 0, 0)
}
