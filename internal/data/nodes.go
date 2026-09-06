package data

// NodeStatus represents the status of a server (node).
type NodeStatus string

const (
	NodeStatusProcessing  NodeStatus = ""
	NodeStatusAvailable              = "available"
	NodeStatusDirty                  = "dirty"
	NodeStatusUnavailable            = "unavailable"
	NodeStatusDisabled               = "disabled"
)

// Node represents a server (node) in the system.
type Node struct {
	Id          string     `json:"id" validate:"required,uuid"`
	Host        string     `json:"host" validate:"required,max=128"`
	HttpToken   string     `json:"http_token"`
	HttpPort    int        `json:"http_port" validate:"omitempty,min=1,max=65535"`
	SshUser     string     `json:"ssh_user" validate:"required"`
	SshPort     int        `json:"ssh_port" validate:"required,min=1,max=65535"`
	SshEnabled  bool       `json:"ssh_enabled"`
	SshStatus   NodeStatus `json:"ssh_status"`
	Usage       float64    `json:"usage" validate:"min=0"`
	UsageBytes  int64      `json:"usage_bytes" validate:"min=0"`
	PushEnabled bool       `json:"push_enabled"`
	PushStatus  NodeStatus `json:"push_status"`
	PushedAt    int64      `json:"pushed_at"`
	PulledAt    int64      `json:"pulled_at"`
}

// NewNode creates a new node with the sync options enabled. Pulling has no flag:
// it is driven by the P-Node running its setup command.
func NewNode(id string, host string, httpToken string, httpPort int, sshUser string, sshPort int) *Node {
	return &Node{
		Id:          id,
		Host:        host,
		HttpToken:   httpToken,
		HttpPort:    httpPort,
		SshUser:     sshUser,
		SshPort:     sshPort,
		SshEnabled:  true,
		PushEnabled: true,
	}
}
