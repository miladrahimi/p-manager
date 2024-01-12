package utils

import (
	"github.com/google/uuid"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// FileExist checks if the given file path exists or not.
func FileExist(path string) bool {
	if stat, err := os.Stat(path); os.IsNotExist(err) || stat.IsDir() {
		return false
	}
	return true
}

func IsPortHealthy(host string, port int) bool {
	timeout := time.Second
	conn, _ := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if conn != nil {
		defer func(conn net.Conn) {
			_ = conn.Close()
		}(conn)
		return true
	}
	return false
}

func UUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
