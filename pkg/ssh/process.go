package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/util"
	"go.uber.org/zap"
)

// Process represents an SSH SOCKS proxy process.
type Process struct {
	l        *logger.Logger
	config   *Config
	command  *exec.Cmd
	locker   sync.Mutex
	sshPath  string
	stopChan chan struct{}
	doneChan chan struct{}
	running  bool
}

const defaultRetryDelay = 2 * time.Second

// Start creates and starts an SSH SOCKS proxy process.
func Start(l *logger.Logger, config *Config) (*Process, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	p := &Process{
		l:      l,
		config: config,
	}
	p.stopChan = make(chan struct{})
	p.doneChan = make(chan struct{})
	p.running = true
	currentStop := p.stopChan
	currentDone := p.doneChan
	go p.run(currentStop, currentDone)
	return p, nil
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

func (p *Process) run(stopChan <-chan struct{}, doneChan chan<- struct{}) {
	defer close(doneChan)

	for {
		if p.shouldStop(stopChan) {
			p.l.Info("ssh: stopped")
			return
		}

		if err := p.ensureBinary(); err != nil {
			p.l.Error("ssh: binary not found", zap.Error(errors.WithStack(err)))
			if !p.waitRetry(stopChan) {
				p.l.Info("ssh: stopped")
				return
			}
			continue
		}

		cmd, err := p.startCommand(stopChan)
		if err != nil {
			if p.shouldStop(stopChan) {
				p.l.Info("ssh: stopped")
				return
			}
			p.l.Error("ssh: start failed", zap.Error(errors.WithStack(err)))
			if !p.waitRetry(stopChan) {
				p.l.Info("ssh: stopped")
				return
			}
			continue
		}

		err = cmd.Wait()
		p.clearCommand(cmd)

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

func (p *Process) startCommand(stopChan <-chan struct{}) (*exec.Cmd, error) {
	if !util.PortFree(p.config.LocalPort) {
		return nil, errors.Errorf("ssh: local port %d is not free", p.config.LocalPort)
	}

	target := fmt.Sprintf("%s@%s", p.config.User, p.config.Host)
	args := []string{
		"-D", strconv.Itoa(p.config.LocalPort),
		"-C",
		"-q",
		"-N",
		"-v",
		"-p", strconv.Itoa(p.config.ServerPort),
		target,
	}

	cmd := exec.Command(p.sshPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	p.l.Info(
		"ssh: starting...",
		zap.String("target", target),
		zap.Int("ssh_port", p.config.ServerPort),
		zap.Int("local_port", p.config.LocalPort),
	)

	if err := cmd.Start(); err != nil {
		return nil, errors.WithStack(err)
	}

	p.locker.Lock()
	if p.stopChan != stopChan || !p.running {
		p.locker.Unlock()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, errors.New("ssh: stopped")
	}
	p.command = cmd
	p.locker.Unlock()

	return cmd, nil
}

func (p *Process) clearCommand(cmd *exec.Cmd) {
	p.locker.Lock()
	if p.command == cmd {
		p.command = nil
	}
	p.locker.Unlock()
}

func (p *Process) shouldStop(stopChan <-chan struct{}) bool {
	select {
	case <-stopChan:
		return true
	default:
		return false
	}
}

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

// ensureBinary ensures that the SSH binary exists.
func (p *Process) ensureBinary() error {
	if p.sshPath != "" {
		return nil
	}

	path, err := exec.LookPath("ssh")
	if err != nil {
		return errors.Wrap(err, "ssh: binary not found")
	}
	p.sshPath = path
	return nil
}
