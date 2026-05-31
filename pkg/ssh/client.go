package ssh

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-node/pkg/logger"
	"golang.org/x/net/proxy"
)

// defaultCheckTimeout bounds the whole SOCKS feasibility probe (connect, auth,
// and a forwarding round-trip). It must exceed the ssh ConnectTimeout below.
const defaultCheckTimeout = 15 * time.Second

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

// Check verifies that the node is usable as a SOCKS proxy: it brings up a
// dynamic SOCKS tunnel the same way StartSocks does and opens a connection
// through it. This reflects exactly what P-Manager needs — a working SOCKS
// outbound — rather than the ability to run a remote shell command (which
// fails for tunnel-only accounts such as a nologin shell).
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
		ctx, cancel = context.WithTimeout(ctx, defaultCheckTimeout)
		defer cancel()
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return errors.WithStack(err)
	}
	socksAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))

	args := []string{
		"-D", socksAddr,
		"-N",
		"-o", "ConnectTimeout=10",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "NumberOfPasswordPrompts=0",
		"-p", strconv.Itoa(config.Port),
		fmt.Sprintf("%s@%s", config.User, config.Host),
	}

	cmd := exec.CommandContext(ctx, c.sshPath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err = cmd.Start(); err != nil {
		return errors.WithStack(err)
	}

	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-exited
	}()

	// Wait for the local SOCKS listener to come up. ssh only binds it after
	// authentication succeeds, so a ready listener already implies a good login.
	if err = waitForSocks(ctx, socksAddr, exited); err != nil {
		return errors.WithMessage(err, c.humanizeError(err, stderr.String()))
	}

	// Confirm the proxy can actually open a channel (TCP forwarding allowed) by
	// connecting back to the node's own SSH port through the SOCKS proxy.
	if err = c.probeSocks(ctx, socksAddr, config.Port); err != nil {
		return errors.WithMessage(err, c.humanizeError(err, stderr.String()))
	}

	return nil
}

// waitForSocks blocks until the local SOCKS listener accepts connections, the
// ssh process exits, or the context is done.
func waitForSocks(ctx context.Context, socksAddr string, exited <-chan struct{}) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	for {
		if conn, err := dialer.DialContext(ctx, "tcp", socksAddr); err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("ssh: timed out waiting for socks proxy")
		case <-exited:
			return errors.New("ssh: connection closed before socks proxy was ready")
		case <-ticker.C:
		}
	}
}

// probeSocks opens a connection through the SOCKS proxy to confirm the node
// permits TCP forwarding. It targets the node's own SSH port over loopback,
// which is reachable from the node whenever forwarding is allowed.
func (c *Client) probeSocks(ctx context.Context, socksAddr string, sshPort int) error {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, &net.Dialer{Timeout: 5 * time.Second})
	if err != nil {
		return errors.WithStack(err)
	}

	target := net.JoinHostPort("127.0.0.1", strconv.Itoa(sshPort))

	var conn net.Conn
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		conn, err = cd.DialContext(ctx, "tcp", target)
	} else {
		conn, err = dialer.Dial("tcp", target)
	}
	if err != nil {
		return errors.WithStack(err)
	}
	_ = conn.Close()

	return nil
}

// freeLocalPort finds a free local TCP port for the probe's SOCKS listener.
func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
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
		// Detect a silently dropped tunnel quickly (~45s) and exit, so the
		// supervisor in process.go restarts it. Without these, a dead tunnel
		// keeps its local SOCKS listener open for up to the kernel TCP
		// keepalive (~2h), and Xray keeps load-balancing traffic into it,
		// causing intermittent rr2ssh timeouts.
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=10",
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
	errText := ""
	if err != nil {
		errText = strings.ToLower(err.Error())
	}
	combined := lower + "\n" + errText

	if strings.Contains(combined, "permission denied") || strings.Contains(combined, "authentication failed") {
		return "ssh: authentication failed"
	}
	if strings.Contains(errText, "unable to authenticate") {
		return "ssh: authentication failed"
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(combined, "timed out") {
		return "ssh: connection timed out"
	}
	if strings.Contains(combined, "connection refused") {
		return "ssh: connection refused"
	}
	if strings.Contains(combined, "no route to host") {
		return "ssh: no route to host"
	}
	// SOCKS channel could not be opened (e.g. AllowTcpForwarding no).
	if strings.Contains(combined, "administratively prohibited") ||
		strings.Contains(combined, "not allowed") ||
		strings.Contains(combined, "open failed") {
		return "ssh: socks forwarding not permitted"
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
