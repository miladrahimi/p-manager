package util

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/xtls/xray-core/common/uuid"
)

// Uuid generates Uuid using XraySettings.
func Uuid() string {
	u := uuid.New()
	return u.String()
}

// ShortId returns a short, URL-safe random id (8 lowercase alphanumerics).
// Enough for the handful of nodes a P-Manager holds.
func ShortId() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return Uuid()
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// StableUuid derives a deterministic RFC-4122 UUID from the seed, so configs
// composed independently can agree on an id without sharing state.
func StableUuid(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// RoundFloat rounds float numbers to the given precision.
func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// SafeSumI64 safely sums two int64 values or returns 0 if the result overflows.
func SafeSumI64(a, b int64) int64 {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0
	}
	return a + b
}

// Bytes2GB converts bytes to GB.
func Bytes2GB(bytes int64) float64 {
	if bytes < 0 {
		return 0
	}

	const bytesPerGB = 1073741824 // 1024^3 = 1,073,741,824
	result := float64(bytes) / float64(bytesPerGB)

	return RoundFloat(result, 2)
}

// GB2Bytes converts GB to bytes.
func GB2Bytes(f float64) int64 {
	if math.IsInf(f, 0) || math.IsNaN(f) || f < 0 {
		return 0
	}

	const bytesPerGB = 1073741824 // 1024^3
	result := f * float64(bytesPerGB)

	if math.IsInf(result, 0) || result > float64(math.MaxInt64) {
		return 0
	}

	return int64(result)
}

// PortsDistinct makes sure all ports are unique or zero (disabled).
func PortsDistinct(ports []int) bool {
	seen := make(map[int]bool)
	for _, port := range ports {
		if port != 0 {
			if seen[port] {
				return false
			}
			seen[port] = true
		}
	}
	return true
}
