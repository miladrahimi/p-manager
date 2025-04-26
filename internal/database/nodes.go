package database

// NodeStatus represents the status of a server (node).
type NodeStatus string

const (
	NodeStatusProcessing  NodeStatus = "processing"
	NodeStatusAvailable              = "available"
	NodeStatusDirty                  = "dirty"
	NodeStatusUnavailable            = "unavailable"
)

// Node represents a server (node) in the system.
type Node struct {
	Id        int        `json:"id"`
	Host      string     `json:"host" validate:"required,max=128"`
	HttpToken string     `json:"http_token" validate:"required"`
	HttpPort  int        `json:"http_port" validate:"required,min=1,max=65536"`
	Usage     float64    `json:"usage"`
	Status    NodeStatus `json:"status"`
}
