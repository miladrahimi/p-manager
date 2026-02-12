package ssh

import (
	"github.com/cockroachdb/errors"
)

// Config represents the SSH proxy configuration.
type Config struct {
	Host       string
	User       string
	ServerPort int
	LocalPort  int
}

// NewConfig creates a new SSH config.
func NewConfig(host, user string, serverPort, localPort int) *Config {
	return &Config{
		Host:       host,
		User:       user,
		ServerPort: serverPort,
		LocalPort:  localPort,
	}
}

// Validate checks the config.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("ssh: config is nil")
	}
	if c.Host == "" {
		return errors.New("ssh: host is required")
	}
	if c.User == "" {
		return errors.New("ssh: user is required")
	}
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return errors.New("ssh: server port is invalid")
	}
	if c.LocalPort < 1 || c.LocalPort > 65535 {
		return errors.New("ssh: local port is invalid")
	}
	return nil
}
