// Package provisioner creates cloud servers, installs P-Node on them, and
// registers them as nodes. Only Hetzner Cloud is supported.
package provisioner

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/hetzner"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
)

// jobTimeout bounds a whole provisioning run (server creation + P-Node install).
const jobTimeout = 30 * time.Minute

var (
	ErrTokenMissing = errors.New("the Hetzner API token is not set in the main settings")
	ErrBusy         = errors.New("a node is already being provisioned")
	ErrMaxNodes     = errors.New("cannot add more nodes")
)

// JobStatus is the state of a provisioning job.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusFailed  JobStatus = "failed"
)

// Job tracks one provisioning run. Jobs live in memory only: a finished job
// is dropped (the node is then listed like any other), a failed one stays
// until dismissed so the error can be read.
type Job struct {
	Id        string    `json:"id"`
	Status    JobStatus `json:"status"`
	Step      string    `json:"step"`
	Error     string    `json:"error,omitempty"`
	StartedAt int64     `json:"started_at"`
	UpdatedAt int64     `json:"updated_at"`
}

// Provisioner runs provisioning jobs and deletes provider servers.
type Provisioner struct {
	l           *logger.Logger
	db          *data.Store
	ssh         *ssh.Client
	coordinator *coordinator.Coordinator

	mu   sync.Mutex
	jobs map[string]*Job
}

// New creates a provisioner.
func New(l *logger.Logger, db *data.Store, sshClient *ssh.Client, c *coordinator.Coordinator) *Provisioner {
	return &Provisioner{l: l, db: db, ssh: sshClient, coordinator: c, jobs: map[string]*Job{}}
}

// Configured reports whether a Hetzner token is set.
func (p *Provisioner) Configured() bool {
	return p.token() != ""
}

// Jobs returns a snapshot of the jobs, oldest first.
func (p *Provisioner) Jobs() []Job {
	p.mu.Lock()
	defer p.mu.Unlock()
	jobs := make([]Job, 0, len(p.jobs))
	for _, j := range p.jobs {
		jobs = append(jobs, *j)
	}
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt < jobs[k].StartedAt })
	return jobs
}

// Dismiss removes a finished (failed) job. Running jobs cannot be dismissed.
func (p *Provisioner) Dismiss(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if j, ok := p.jobs[id]; ok && j.Status != JobStatusRunning {
		delete(p.jobs, id)
	}
}

// Start begins provisioning a Hetzner node in the background. managerUrl (the
// public base URL of this P-Manager) is used to configure pulling on the new
// node; empty skips that step.
func (p *Provisioner) Start(managerUrl string) (Job, error) {
	token := p.token()
	if token == "" {
		return Job{}, ErrTokenMissing
	}
	var count int
	p.db.Read(func(d *data.Data) { count = len(d.Nodes) })
	if count >= config.MaxNodesCount {
		return Job{}, ErrMaxNodes
	}

	p.mu.Lock()
	for _, j := range p.jobs {
		if j.Status == JobStatusRunning {
			p.mu.Unlock()
			return Job{}, ErrBusy
		}
	}
	now := time.Now().UnixMilli()
	job := &Job{Id: util.ShortId(), Status: JobStatusRunning, Step: "Starting", StartedAt: now, UpdatedAt: now}
	p.jobs[job.Id] = job
	p.mu.Unlock()

	go p.run(job, token, managerUrl)
	return *job, nil
}

// Destroy deletes the provider server backing the node, if any. It is a no-op
// for manually added nodes and for servers that are already gone.
func (p *Provisioner) Destroy(node data.Node) error {
	if node.Provider != data.NodeProviderHetzner || node.ProviderServerId == 0 {
		return nil
	}
	token := p.token()
	if token == "" {
		return ErrTokenMissing
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err := hetzner.New(token).DeleteServer(ctx, node.ProviderServerId)
	if err != nil && !hetzner.IsNotFound(err) {
		return errors.WithStack(err)
	}
	p.l.Info("provisioner: hetzner server deleted", zap.String("node", node.Id), zap.Int64("server", node.ProviderServerId))
	return nil
}

// run executes a job and records its outcome. A server created by a failed
// run is deleted again so it does not keep billing.
func (p *Provisioner) run(job *Job, token, managerUrl string) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	client := hetzner.New(token)
	server, err := p.provision(ctx, job, client, managerUrl)
	if err == nil {
		p.l.Info("provisioner: node provisioned", zap.String("job", job.Id), zap.String("server", server.Name))
		p.mu.Lock()
		delete(p.jobs, job.Id)
		p.mu.Unlock()
		return
	}

	message := fmt.Sprintf("%s: %s", p.step(job), err.Error())
	if server != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		if cleanupErr := client.DeleteServer(cleanupCtx, server.Id); cleanupErr != nil && !hetzner.IsNotFound(cleanupErr) {
			message += fmt.Sprintf(" (the Hetzner server %s could not be deleted, remove it manually: %s)", server.Name, cleanupErr.Error())
		}
		cleanupCancel()
	}
	p.l.Error("provisioner: job failed", zap.String("job", job.Id), zap.Error(err))

	p.mu.Lock()
	job.Status = JobStatusFailed
	job.Error = message
	job.UpdatedAt = time.Now().UnixMilli()
	p.mu.Unlock()
}

// setStep records the job's current step.
func (p *Provisioner) setStep(job *Job, step string) {
	p.mu.Lock()
	job.Step = step
	job.UpdatedAt = time.Now().UnixMilli()
	p.mu.Unlock()
	p.l.Info("provisioner: "+step, zap.String("job", job.Id))
}

func (p *Provisioner) step(job *Job) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return job.Step
}

func (p *Provisioner) token() string {
	var token string
	p.db.Read(func(d *data.Data) { token = d.MainSettings.HetznerToken })
	return token
}

// newNodeId returns a node id not used by any existing node.
func (p *Provisioner) newNodeId() string {
	id := util.ShortId()
	p.db.Read(func(d *data.Data) {
		for d.FindNodeById(id) != nil {
			id = util.ShortId()
		}
	})
	return id
}

// addNode registers the provisioned server as a node.
func (p *Provisioner) addNode(id string, server *hetzner.Server, info nodeInfo, details map[string]string) (data.Node, error) {
	var node data.Node
	err := p.db.Mutate(func(d *data.Data) (bool, error) {
		if len(d.Nodes) >= config.MaxNodesCount {
			return false, ErrMaxNodes
		}
		if d.FindNodeById(id) != nil {
			return false, errors.Newf("node id %s is already taken", id)
		}
		n := data.NewNode(id, server.Ipv4(), info.HttpToken, info.HttpPort, nodeSshUser, nodeSshPort)
		n.SshStatus = data.NodeStatusProcessing
		n.Provider = data.NodeProviderHetzner
		n.ProviderServerId = server.Id
		n.Details = details
		d.Nodes = append(d.Nodes, n)
		node = *n
		return true, nil
	})
	return node, errors.WithStack(err)
}
