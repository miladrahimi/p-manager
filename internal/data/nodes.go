package data

// NodeStatus represents the status of a server (node).
type NodeStatus string

const (
	NodeStatusProcessing  NodeStatus = ""
	NodeStatusAvailable              = "available"
	NodeStatusDirty                  = "dirty"
	NodeStatusUnavailable            = "unavailable"
)

// Node represents a server (node) in the system.
type Node struct {
	Id         int        `json:"id"`
	Host       string     `json:"host" validate:"required,max=128"`
	HttpToken  string     `json:"http_token" validate:"required"`
	HttpPort   int        `json:"http_port" validate:"required,min=1,max=65535"`
	Usage      float64    `json:"usage" validate:"min=0"`
	UsageBytes int64      `json:"usage_bytes" validate:"min=0"`
	PushStatus NodeStatus `json:"push_status"`
	PullStatus NodeStatus `json:"pull_status"`
	PushedAt   int64      `json:"pushed_at"`
	PulledAt   int64      `json:"pulled_at"`
}

// NewNode creates a new node instance.
func NewNode(id int, host string, httpToken string, httpPort int) *Node {
	return &Node{
		Id:        id,
		Host:      host,
		HttpToken: httpToken,
		HttpPort:  httpPort,
	}
}
