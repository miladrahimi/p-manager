package ssh

import (
	"strings"

	"github.com/cockroachdb/errors"
)

// Config represents the SSH proxy configuration.
type Config struct {
	Host      string
	User      string
	SshPort   int
	LocalPort int
}

// NewConfig creates a new SSH config.
func NewConfig(host, user string, sshPort, localPort int) *Config {
	return &Config{
		Host:      host,
		User:      user,
		SshPort:   sshPort,
		LocalPort: localPort,
	}
}

// Validate checks the config.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("ssh: config is nil")
	}
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("ssh: host is required")
	}
	if strings.TrimSpace(c.User) == "" {
		return errors.New("ssh: user is required")
	}
	if c.SshPort < 1 || c.SshPort > 65535 {
		return errors.New("ssh: ssh port is invalid")
	}
	if c.LocalPort < 1 || c.LocalPort > 65535 {
		return errors.New("ssh: local port is invalid")
	}
	return nil
}
