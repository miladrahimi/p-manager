package utils

import (
	"fmt"
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

// PortAvailable checks if the TCP port is reachable or not.
func PortAvailable(host string, port int) bool {
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

	defer func() {
		err = listener.Close()
	}()

	return listener.Addr().(*net.TCPAddr).Port, err
}

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
