package ssh

import "github.com/cockroachdb/errors"

// ConnectionConfig represents the SSH client configuration.
type ConnectionConfig struct {
	Host string
	User string
	Port int
}

// NewConnectionConfig creates a new SSH connection config.
func NewConnectionConfig(host, user string, port int) *ConnectionConfig {
	return &ConnectionConfig{
		Host: host,
		User: user,
		Port: port,
	}
}

// Validate checks the config.
func (c *ConnectionConfig) Validate() error {
	if c == nil {
		return errors.New("ssh: config is nil")
	}
	if c.Host == "" {
		return errors.New("ssh: host is required")
	}
	if c.User == "" {
		return errors.New("ssh: user is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("ssh: server port is invalid")
	}
	return nil
}
