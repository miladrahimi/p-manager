package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/util"
	"go.uber.org/zap"
)

// Process represents an SSH SOCKS proxy process.
type Process struct {
	l          *logger.Logger
	config     *ProxyConfig
	buildCmd   func() (*exec.Cmd, func(), error)
	command    *exec.Cmd
	locker     sync.Mutex
	stdoutPath string
	stderrPath string
	stopChan   chan struct{}
	doneChan   chan struct{}
	running    bool
}

const (
	// minRetryDelay is the pause before restarting a tunnel that exited.
	minRetryDelay = 2 * time.Second
	// maxRetryDelay caps the doubling pause while a node stays unreachable.
	maxRetryDelay = 30 * time.Second
	// stableRun is how long a tunnel must stay up to count as healthy again. It
	// is longer than the ServerAlive window (15 s x 3), so a tunnel that comes
	// up and then dies on its keepalive still counts as a failed attempt.
	stableRun = 2 * time.Minute
)

// nextRetryDelay returns the pause before the next start: minRetryDelay after a
// stable run (or the first one), else double the previous pause up to
// maxRetryDelay. A down node used to cost every tunnel a restart (and an error
// line) every 2-12 s; with 16 tunnels that is thousands of lines an hour.
func nextRetryDelay(prev, ran time.Duration) time.Duration {
	if prev <= 0 || ran >= stableRun {
		return minRetryDelay
	}
	return min(prev*2, maxRetryDelay)
}

func newProcess(
	l *logger.Logger,
	config *ProxyConfig,
	stdoutPath string,
	stderrPath string,
	buildCmd func() (*exec.Cmd, func(), error),
) *Process {
	p := &Process{
		l:          l,
		config:     config,
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		buildCmd:   buildCmd,
	}
	p.stopChan = make(chan struct{})
	p.doneChan = make(chan struct{})
	p.running = true
	currentStop := p.stopChan
	currentDone := p.doneChan
	go p.run(currentStop, currentDone)
	return p
}

// Stop stops the SSH proxy.
func (p *Process) Stop() error {
	p.locker.Lock()
	if !p.running {
		p.locker.Unlock()
		return nil
	}

	stopChan := p.stopChan
	doneChan := p.doneChan
	cmd := p.command
	p.running = false
	p.stopChan = nil
	p.doneChan = nil
	p.command = nil
	p.locker.Unlock()

	if stopChan != nil {
		close(stopChan)
	}

	if cmd != nil && cmd.Process != nil {
		p.l.Debug("ssh: stopping...")
		if err := cmd.Process.Kill(); err != nil {
			return errors.WithStack(err)
		}
	}

	if doneChan != nil {
		select {
		case <-doneChan:
		case <-time.After(5 * time.Second):
			p.l.Debug("ssh: stop timed out")
		}
	}

	return nil
}

// run runs the SSH SOCKS proxy process and restarts it when it exits, backing
// off while it keeps failing. Only the first failure of a streak is logged as
// an error; repeats are debug lines, so an outage costs one line per tunnel.
func (p *Process) run(stopChan <-chan struct{}, doneChan chan<- struct{}) {
	defer close(doneChan)

	var delay time.Duration
	failures := 0
	for {
		if p.shouldStop(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}

		started := time.Now()
		message := "ssh: process exited"
		cmd, cleanup, stdoutFile, stderrFile, err := p.startCommand(stopChan)
		if err != nil {
			message = "ssh: start failed"
		} else {
			err = cmd.Wait()
			p.clearCommand(cmd)
			p.closeOutputFiles(stdoutFile, stderrFile)
		}
		if cleanup != nil {
			cleanup()
		}

		if p.shouldStop(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}

		ran := time.Since(started)
		delay = nextRetryDelay(delay, ran)
		if ran >= stableRun {
			failures = 0
		}

		if err == nil || err.Error() == "signal: killed" {
			p.l.Info("ssh: exited", p.fields(ran, delay)...)
		} else {
			failures++
			fields := append(p.fields(ran, delay), zap.String("error", err.Error()), zap.Int("failures", failures))
			if failures == 1 {
				p.l.Error(message, fields...)
			} else {
				p.l.Debug(message, fields...)
			}
		}

		if !p.waitRetry(stopChan, delay) {
			p.l.Info("ssh: stopped")
			return
		}
	}
}

// fields describes the tunnel in log lines (no stack trace: an exit status
// carries none worth reading).
func (p *Process) fields(ran, retryIn time.Duration) []zap.Field {
	fields := []zap.Field{zap.Duration("ran", ran.Round(time.Second)), zap.Duration("retry_in", retryIn)}
	if p.config != nil {
		fields = append(fields, zap.Int("local_port", p.config.LocalPort))
		if p.config.Connection != nil {
			fields = append(fields, zap.String("host", p.config.Connection.Host))
		}
	}
	return fields
}

// startCommand starts the SSH SOCKS proxy command.
func (p *Process) startCommand(stopChan <-chan struct{}) (*exec.Cmd, func(), *os.File, *os.File, error) {
	if p.config == nil {
		return nil, nil, nil, nil, errors.New("ssh: config is nil")
	}
	if !util.PortFree(p.config.LocalPort) {
		return nil, nil, nil, nil, errors.Errorf("ssh: local port %d is not free", p.config.LocalPort)
	}
	if p.buildCmd == nil {
		return nil, nil, nil, nil, errors.New("ssh: command factory is nil")
	}

	cmd, cleanup, err := p.buildCmd()
	if err != nil {
		return nil, nil, nil, nil, errors.WithStack(err)
	}

	stdoutFile, stderrFile, err := p.openLogFiles()
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, nil, nil, errors.WithStack(err)
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		p.closeOutputFiles(stdoutFile, stderrFile)
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, nil, nil, errors.WithStack(err)
	}

	p.locker.Lock()
	if p.stopChan != stopChan || !p.running {
		p.locker.Unlock()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		p.closeOutputFiles(stdoutFile, stderrFile)
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, nil, nil, errors.New("ssh: stopped")
	}
	p.command = cmd
	p.locker.Unlock()

	return cmd, cleanup, stdoutFile, stderrFile, nil
}

// clearCommand clears the command reference if it matches the provided command.
func (p *Process) clearCommand(cmd *exec.Cmd) {
	p.locker.Lock()
	if p.command == cmd {
		p.command = nil
	}
	p.locker.Unlock()
}

// shouldStop checks if the stop channel is closed.
func (p *Process) shouldStop(stopChan <-chan struct{}) bool {
	select {
	case <-stopChan:
		return true
	default:
		return false
	}
}

// waitRetry waits for the given delay or until the stop channel is closed.
func (p *Process) waitRetry(stopChan <-chan struct{}, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stopChan:
		return false
	case <-timer.C:
		return true
	}
}

// openLogFiles opens the log files for the SSH proxy output.
func (p *Process) openLogFiles() (*os.File, *os.File, error) {
	stdoutFile, err := p.openLogFile(p.stdoutPath)
	if err != nil {
		return nil, nil, err
	}
	if p.stderrPath == p.stdoutPath {
		return stdoutFile, stdoutFile, nil
	}

	stderrFile, err := p.openLogFile(p.stderrPath)
	if err != nil {
		_ = stdoutFile.Close()
		return nil, nil, err
	}

	return stdoutFile, stderrFile, nil
}

// openLogFile opens the log file for the SSH proxy output.
func (p *Process) openLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return file, nil
}

// closeOutputFiles closes the log files for the SSH proxy output.
func (p *Process) closeOutputFiles(stdoutFile, stderrFile *os.File) {
	if stdoutFile != nil {
		_ = stdoutFile.Close()
	}
	if stderrFile != nil && stderrFile != stdoutFile {
		_ = stderrFile.Close()
	}
}
