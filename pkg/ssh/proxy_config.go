package ssh

import "github.com/cockroachdb/errors"

// ProxyConfig represents the SSH SOCKS proxy configuration.
type ProxyConfig struct {
	Connection *ConnectionConfig
	LocalPort  int
}

// NewProxyConfig creates a new SSH proxy config.
func NewProxyConfig(connection *ConnectionConfig, localPort int) *ProxyConfig {
	return &ProxyConfig{
		Connection: connection,
		LocalPort:  localPort,
	}
}

// Validate checks the proxy config.
func (c *ProxyConfig) Validate() error {
	if c == nil {
		return errors.New("ssh: proxy config is nil")
	}
	if c.Connection == nil {
		return errors.New("ssh: connection config is nil")
	}
	if err := c.Connection.Validate(); err != nil {
		return err
	}
	if c.LocalPort < 1 || c.LocalPort > 65535 {
		return errors.New("ssh: local port is invalid")
	}
	return nil
}
