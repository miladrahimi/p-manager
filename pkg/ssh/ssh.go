package ssh

import (
	"context"
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

// Ssh represents an SSH SOCKS proxy instance.
type Ssh struct {
	l            *logger.Logger
	config       *Config
	command      *exec.Cmd
	locker       sync.Mutex
	context      context.Context
	sshPath      string
	stopChan     chan struct{}
	doneChan     chan struct{}
	restartDelay time.Duration
	running      bool
}

// newProxy creates a new SSH proxy instance.
func newProxy(c context.Context, l *logger.Logger, config *Config) *Ssh {
	return &Ssh{
		context:      c,
		l:            l,
		config:       config,
		restartDelay: 2 * time.Second,
	}
}

// Run starts the SSH SOCKS proxy and keeps it alive.
func (s *Ssh) Run() error {
	s.locker.Lock()
	defer s.locker.Unlock()

	if s.running {
		return nil
	}

	if err := s.ensureBinaryLocked(); err != nil {
		return errors.WithStack(err)
	}
	if err := s.config.Validate(); err != nil {
		return errors.WithStack(err)
	}

	s.stopChan = make(chan struct{})
	s.doneChan = make(chan struct{})

	if err := s.startLocked(); err != nil {
		s.stopChan = nil
		s.doneChan = nil
		return errors.WithStack(err)
	}

	s.running = true
	currentStop := s.stopChan
	currentDone := s.doneChan
	currentCmd := s.command
	go s.monitor(currentCmd, currentStop, currentDone)

	return nil
}

// Stop stops the SSH proxy.
func (s *Ssh) Stop() error {
	s.locker.Lock()
	if !s.running {
		s.locker.Unlock()
		return nil
	}

	stopChan := s.stopChan
	doneChan := s.doneChan
	cmd := s.command
	s.running = false
	s.stopChan = nil
	s.doneChan = nil
	s.command = nil
	s.locker.Unlock()

	if stopChan != nil {
		close(stopChan)
	}

	if cmd != nil && cmd.Process != nil {
		s.l.Debug("ssh: stopping...")
		if err := cmd.Process.Kill(); err != nil {
			return errors.WithStack(err)
		}
	}

	if doneChan != nil {
		select {
		case <-doneChan:
		case <-time.After(5 * time.Second):
			s.l.Debug("ssh: stop timed out")
		}
	}

	return nil
}

// Restart restarts the SSH proxy.
func (s *Ssh) Restart() error {
	if err := s.Stop(); err != nil {
		return errors.WithStack(err)
	}
	return s.Run()
}

func (s *Ssh) startLocked() error {
	if err := s.config.Validate(); err != nil {
		return errors.WithStack(err)
	}
	if !util.PortFree(s.config.LocalPort) {
		return errors.Errorf("ssh: local port %d is not free", s.config.LocalPort)
	}

	target := fmt.Sprintf("%s@%s", s.config.User, s.config.Host)
	args := []string{
		"-D", strconv.Itoa(s.config.LocalPort),
		"-C",
		"-q",
		"-N",
		"-v",
		"-p", strconv.Itoa(s.config.SshPort),
		target,
	}

	s.command = exec.Command(s.sshPath, args...)
	s.command.Stdout = os.Stdout
	s.command.Stderr = os.Stderr

	s.l.Info(
		"ssh: starting...",
		zap.String("target", target),
		zap.Int("ssh_port", s.config.SshPort),
		zap.Int("local_port", s.config.LocalPort),
	)

	if err := s.command.Start(); err != nil {
		s.command = nil
		return errors.WithStack(err)
	}
	return nil
}

func (s *Ssh) monitor(cmd *exec.Cmd, stopChan <-chan struct{}, doneChan chan<- struct{}) {
	defer close(doneChan)

	for {
		err := cmd.Wait()
		if err != nil && err.Error() != "signal: killed" {
			s.l.Error("ssh: process exited", zap.Error(errors.WithStack(err)))
		}

		select {
		case <-stopChan:
			s.l.Info("ssh: stopped")
			return
		default:
		}
		if s.context.Err() != nil {
			s.l.Info("ssh: context canceled")
			return
		}

		s.l.Info("ssh: restarting...")
		time.Sleep(s.restartDelay)

		for {
			select {
			case <-stopChan:
				s.l.Info("ssh: stopped")
				return
			default:
			}
			if s.context.Err() != nil {
				s.l.Info("ssh: context canceled")
				return
			}

			s.locker.Lock()
			if s.stopChan != stopChan {
				s.locker.Unlock()
				return
			}
			if err := s.startLocked(); err != nil {
				s.locker.Unlock()
				s.l.Error("ssh: cannot restart", zap.Error(errors.WithStack(err)))
				time.Sleep(s.restartDelay)
				continue
			}
			cmd = s.command
			s.locker.Unlock()
			break
		}
	}
}

func (s *Ssh) ensureBinaryLocked() error {
	if s.sshPath != "" {
		return nil
	}

	path, err := exec.LookPath("ssh")
	if err != nil {
		return errors.Wrap(err, "ssh: binary not found")
	}
	s.sshPath = path
	return nil
}
