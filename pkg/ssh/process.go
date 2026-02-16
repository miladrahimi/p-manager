package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/logger"
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

const defaultRetryDelay = 2 * time.Second

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

// run runs the SSH SOCKS proxy process.
func (p *Process) run(stopChan <-chan struct{}, doneChan chan<- struct{}) {
	defer close(doneChan)

	for {
		if p.shouldStop(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}

		cmd, cleanup, stdoutFile, stderrFile, err := p.startCommand(stopChan)
		if err != nil {
			if p.shouldStop(stopChan) {
				p.l.Info("ssh: stopped")
				return
			}
			p.l.Error("ssh: start failed", zap.Error(errors.WithStack(err)))
			if cleanup != nil {
				cleanup()
			}
			if !p.waitRetry(stopChan) {
				p.l.Info("ssh: stopped")
				return
			}
			continue
		}

		err = cmd.Wait()
		p.clearCommand(cmd)
		p.closeOutputFiles(stdoutFile, stderrFile)
		if cleanup != nil {
			cleanup()
		}

		if p.shouldStop(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}

		if err != nil && err.Error() != "signal: killed" {
			p.l.Error("ssh: process exited", zap.Error(errors.WithStack(err)))
		} else {
			p.l.Info("ssh: exited")
		}

		if !p.waitRetry(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}
	}
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

// waitRetry waits for the retry delay or until the stop channel is closed.
func (p *Process) waitRetry(stopChan <-chan struct{}) bool {
	timer := time.NewTimer(defaultRetryDelay)
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
