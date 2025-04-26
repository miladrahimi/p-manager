package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"math"
	"net"
	"os"
	"strings"
)

// FileExist checks if the given file path exists or not.
func FileExist(path string) bool {
	if stat, err := os.Stat(path); os.IsNotExist(err) || stat.IsDir() {
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

// UUID generates UUID without the '-' character.
func UUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// RoundFloat rounds float numbers to the given precision.
func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
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
		return PortFree(port)
	}

	return true
}

// PortsUnique makes sure all ports are unique or zero (disabled).
func PortsUnique(ports []int) bool {
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
