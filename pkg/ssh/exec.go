package ssh

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// Run executes a shell command on the remote host over SSH and returns its
// combined output. The caller bounds it with the context.
func (c *Client) Run(ctx context.Context, config *ConnectionConfig, command string) (string, error) {
	if err := config.Validate(); err != nil {
		return "", errors.WithStack(err)
	}
	if err := c.ensureBinary(); err != nil {
		return "", errors.WithStack(err)
	}

	args := []string{
		"-o", "ConnectTimeout=10",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "NumberOfPasswordPrompts=0",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=4",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(config.Port),
		fmt.Sprintf("%s@%s", config.User, config.Host),
		command,
	}

	out, err := exec.CommandContext(ctx, c.sshPath, args...).CombinedOutput()
	output := string(out)
	if err != nil {
		if ctx.Err() != nil {
			return output, errors.Wrap(ctx.Err(), "ssh: command interrupted")
		}
		return output, errors.Wrapf(err, "ssh: command failed: %s", tail(output, 600))
	}
	return output, nil
}

// ForgetHost drops any known_hosts entry for the host, so a re-used address
// presenting a new host key does not trip ssh's host-key-changed protection
// (which silently disables port forwarding).
func (c *Client) ForgetHost(ctx context.Context, host string) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return
	}
	_ = exec.CommandContext(ctx, keygen, "-R", host).Run()
}

// tail returns the trimmed last n bytes of s.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "..." + s[len(s)-n:]
	}
	return s
}
