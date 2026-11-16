package data

import (
	"encoding/json"
	"net/netip"

	"github.com/miladrahimi/p-manager/pkg/util"
)

// NodeStatus represents the status of a server (node).
type NodeStatus string

const (
	NodeStatusProcessing  NodeStatus = ""
	NodeStatusAvailable   NodeStatus = "available"
	NodeStatusDirty       NodeStatus = "dirty"
	NodeStatusUnavailable NodeStatus = "unavailable"
	NodeStatusDisabled    NodeStatus = "disabled"
)

// NodeProviderHetzner marks a node whose server P-Manager created on Hetzner
// Cloud; deleting such a node deletes the server first.
const NodeProviderHetzner = "hetzner"

// Node represents a server (node) in the system.
type Node struct {
	Id          string     `json:"id" validate:"required,max=64"`
	Host        string     `json:"host" validate:"required,max=128"`
	PullToken   string     `json:"pull_token"`
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
	// AutoIpEnabled lets the node's pulls move its host to the IP they come
	// from (see AutoIpHost), so a P-Node whose server got a new IP follows it.
	AutoIpEnabled bool `json:"auto_ip_enabled"`
	// Provider is empty for manually added nodes, or the cloud provider whose
	// server backs this node (NodeProviderHetzner), with its server id.
	Provider         string `json:"provider" validate:"omitempty,oneof=hetzner"`
	ProviderServerId int64  `json:"provider_server_id"`
	// Details holds free-form information about the node shown in the UI
	// (e.g. the Hetzner server specs and price), keyed by label.
	Details map[string]string `json:"details"`
}

// NewNode creates a new node with the sync options enabled. Pulling has no flag:
// it is driven by the P-Node running its setup command.
func NewNode(id string, host string, httpToken string, httpPort int, sshUser string, sshPort int) *Node {
	return &Node{
		Id:            id,
		Host:          host,
		PullToken:     util.Uuid(),
		HttpToken:     httpToken,
		HttpPort:      httpPort,
		SshUser:       sshUser,
		SshPort:       sshPort,
		SshEnabled:    true,
		PushEnabled:   true,
		AutoIpEnabled: true,
		Details:       map[string]string{},
	}
}

// UnmarshalJSON turns Auto IP on for nodes stored before the flag existed.
func (n *Node) UnmarshalJSON(b []byte) error {
	type plain Node
	p := plain{AutoIpEnabled: true}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*n = Node(p)
	return nil
}

// AutoIpHost returns the host Auto IP gives the node after a pull from ip and
// whether it differs from the current one. Only an IP host follows its pulls,
// and only to an address of the same family and kind (public, private or
// loopback): a pull seen through a proxy in front of P-Manager (a loopback or
// private source) must not overwrite a public address.
func (n *Node) AutoIpHost(ip string) (string, bool) {
	if !n.AutoIpEnabled {
		return n.Host, false
	}
	current, err := netip.ParseAddr(n.Host)
	if err != nil {
		return n.Host, false // a domain name
	}
	pulled, err := netip.ParseAddr(ip)
	if err != nil {
		return n.Host, false
	}
	current, pulled = current.Unmap(), pulled.Unmap()
	if pulled == current || ipKind(pulled) == "" || ipKind(pulled) != ipKind(current) {
		return n.Host, false
	}
	return pulled.String(), true
}

// ipKind classifies an address by family and reach; "" for one that is never a
// node's host (unspecified, multicast).
func ipKind(a netip.Addr) string {
	family := "ipv6"
	if a.Is4() {
		family = "ipv4"
	}
	switch {
	case a.IsLoopback():
		return family + "-loopback"
	case a.IsPrivate() || a.IsLinkLocalUnicast():
		return family + "-private"
	case a.IsGlobalUnicast():
		return family + "-public"
	}
	return ""
}
