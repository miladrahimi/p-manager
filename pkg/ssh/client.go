package ssh

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-node/pkg/logger"
)

const defaultConnectTimeout = 5 * time.Second

// Client is an SSH client backed by the system ssh binary.
type Client struct {
	logger  *logger.Logger
	sshPath string
}

// NewClient creates a new SSH client.
func NewClient(l *logger.Logger) (*Client, error) {
	if l == nil {
		return nil, errors.New("ssh: logger is nil")
	}

	client := &Client{logger: l}
	if err := client.ensureBinary(); err != nil {
		return nil, errors.WithStack(err)
	}
	return client, nil
}

// Check verifies that the SSH connection can be established.
func (c *Client) Check(ctx context.Context, config *ConnectionConfig) error {
	if err := config.Validate(); err != nil {
		return errors.WithStack(err)
	}
	if err := c.ensureBinary(); err != nil {
		return errors.WithStack(err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultConnectTimeout)
		defer cancel()
	}

	args := []string{
		"-o", "ConnectTimeout=5",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "NumberOfPasswordPrompts=0",
		"-p", strconv.Itoa(config.Port),
		fmt.Sprintf("%s@%s", config.User, config.Host),
		"exit", "0",
	}

	cmd := exec.CommandContext(ctx, c.sshPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithMessage(err, c.humanizeError(err, string(output)))
	}
	return nil
}

// StartSocks starts a SOCKS proxy process for the given config.
func (c *Client) StartSocks(config *ProxyConfig, stdoutPath, stderrPath string) (*Process, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := c.ensureBinary(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []string{
		"-D", strconv.Itoa(config.LocalPort),
		"-C",
		"-q",
		"-N",
		"-v",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "NumberOfPasswordPrompts=0",
		"-p", strconv.Itoa(config.Connection.Port),
		fmt.Sprintf("%s@%s", config.Connection.User, config.Connection.Host),
	}

	buildCmd := func() (*exec.Cmd, func(), error) {
		cmd := exec.Command(c.sshPath, args...)
		return cmd, nil, nil
	}

	return newProcess(c.logger, config, stdoutPath, stderrPath, buildCmd), nil
}

func (c *Client) humanizeError(err error, output string) string {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "authentication failed") {
		return "ssh: authentication failed"
	}
	if strings.Contains(strings.ToLower(err.Error()), "unable to authenticate") {
		return "ssh: authentication failed"
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timed out") {
		return "ssh: connection timed out"
	}
	if strings.Contains(lower, "connection refused") {
		return "ssh: connection refused"
	}
	if strings.Contains(lower, "no route to host") {
		return "ssh: no route to host"
	}
	if strings.TrimSpace(output) != "" {
		return fmt.Sprintf("ssh: check failed: %s", strings.TrimSpace(output))
	}
	return "ssh: check failed"
}

func (c *Client) ensureBinary() error {
	if c.sshPath != "" {
		return nil
	}

	path, err := exec.LookPath("ssh")
	if err != nil {
		return errors.Wrap(err, "ssh: binary not found")
	}
	c.sshPath = path
	return nil
}
