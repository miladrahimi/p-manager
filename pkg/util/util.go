package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"os"

	"github.com/xtls/xray-core/common/uuid"
)

// FileExist checks if the given file path exists or not.
func FileExist(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	if stat.IsDir() {
		return false
	}
	return true
}

// Key32 generates 32-bit keys.
func Key32() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Uuid generates Uuid using XraySettings.
func Uuid() string {
	u := uuid.New()
	return u.String()
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

// FreePort finds a free port.
func FreePort() (int, error) {
	address, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	listener, err := net.ListenTCP("tcp", address)
	if err != nil {
		return 0, err
	}
	if err = listener.Close(); err != nil {
		return 0, err
	}

	return listener.Addr().(*net.TCPAddr).Port, err
}

// PortFree checks if the given port is free or not.
func PortFree(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}

	if err = listener.Close(); err != nil {
		return false
	}

	return true
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
