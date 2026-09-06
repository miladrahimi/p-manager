package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
)

// pullStaleAfter bounds how long since the last pull counts as available
// (P-Nodes pull every 30s).
const pullStaleAfter = 90 * time.Second

type NodeResponse struct {
	data.Node
	PullStatus  data.NodeStatus `json:"pull_status"`
	PullCommand string          `json:"pull_command"`
}

// pullStatus derives a node's pull status from its last pull time.
func pullStatus(pulledAt int64) data.NodeStatus {
	if pulledAt == 0 {
		return data.NodeStatusProcessing
	}
	if time.Since(time.UnixMilli(pulledAt)) <= pullStaleAfter {
		return data.NodeStatusAvailable
	}
	return data.NodeStatusUnavailable
}

type NodesStoreRequest struct {
	Host        string `json:"host" validate:"required,max=128"`
	HttpToken   string `json:"http_token"`
	HttpPort    int    `json:"http_port" validate:"omitempty,min=1,max=65535"`
	SshUser     string `json:"ssh_user" validate:"required"`
	SshPort     int    `json:"ssh_port" validate:"required,min=1,max=65535"`
	SshEnabled  *bool  `json:"ssh_enabled"`
	PushEnabled *bool  `json:"push_enabled"`
}

// pushHttpMissing reports whether push is enabled but its HTTP credentials are
// absent. HTTP token/port are optional when push is disabled.
func pushHttpMissing(pushEnabled bool, token string, port int) bool {
	return pushEnabled && (token == "" || port < 1)
}

type NodesUpdateRequest struct {
	NodesStoreRequest
}

type NodesUpdatePartialRequest struct {
	Usage *float64 `json:"usage"`
}

type NodesTogglesRequest struct {
	SshEnabled  *bool `json:"ssh_enabled"`
	PushEnabled *bool `json:"push_enabled"`
}

// boolOr returns the pointed-to value or the given default when nil. Requests
// omitting a sync flag (e.g. the P-Node JSON paste flow) default to enabled.
func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// NodesIndex returns the list of nodes.
func NodesIndex(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var response []NodeResponse
		db.Read(func(d *data.Data) {
			token := d.MainSettings.AdminPassword
			response = make([]NodeResponse, 0, len(d.Nodes))
			for _, node := range d.Nodes {
				cmd := fmt.Sprintf("make set-manager URL=\"BASE_URL/v1/nodes/%s\" TOKEN=\"%s\"", node.Id, token)
				n := *node
				if !n.SshEnabled {
					n.SshStatus = data.NodeStatusDisabled
				}
				if !n.PushEnabled {
					n.PushStatus = data.NodeStatusDisabled
				}
				response = append(response, NodeResponse{
					Node:        n,
					PullStatus:  pullStatus(n.PulledAt),
					PullCommand: cmd,
				})
			}
		})

		return c.JSON(http.StatusOK, response)
	}
}

// NodesStore stores a new node.
func NodesStore(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesStoreRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}
		if pushHttpMissing(boolOr(r.PushEnabled, true), r.HttpToken, r.HttpPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "HTTP token and port are required when push is enabled.",
			})
		}

		var node data.Node
		var nodeId string
		var failure *httpResult
		err := db.Mutate(func(d *data.Data) (bool, error) {
			if len(d.Nodes) > 5 {
				failure = &httpResult{http.StatusForbidden, "Cannot add more nodes!"}
				return false, nil
			}

			var target *data.Node
			for _, n := range d.Nodes {
				if n.Host == r.Host && n.HttpPort == r.HttpPort {
					target = n
					target.HttpToken = r.HttpToken
					target.SshUser = r.SshUser
					target.SshPort = r.SshPort
					target.SshStatus = data.NodeStatusProcessing
				}
			}
			if target == nil {
				target = data.NewNode(util.Uuid(), r.Host, r.HttpToken, r.HttpPort, r.SshUser, r.SshPort)
				target.SshStatus = data.NodeStatusProcessing
				d.Nodes = append(d.Nodes, target)
			}

			target.SshEnabled = boolOr(r.SshEnabled, target.SshEnabled)
			target.PushEnabled = boolOr(r.PushEnabled, target.PushEnabled)

			nodeId = target.Id
			node = *target
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if failure != nil {
			return failure.write(c)
		}

		go coordinator.CheckSshStatus(nodeId)
		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, node)
	}
}

// NodesUpdate updates a node.
func NodesUpdate(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesUpdateRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}
		if pushHttpMissing(boolOr(r.PushEnabled, true), r.HttpToken, r.HttpPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "HTTP token and port are required when push is enabled.",
			})
		}

		var node data.Node
		var nodeId string
		found := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			var target *data.Node
			for _, n := range d.Nodes {
				if n.Id == c.Param("id") {
					target = n
				}
			}
			if target == nil {
				return false, nil
			}

			target.Host = r.Host
			target.HttpToken = r.HttpToken
			target.HttpPort = r.HttpPort
			target.SshUser = r.SshUser
			target.SshPort = r.SshPort
			target.SshStatus = data.NodeStatusProcessing
			target.SshEnabled = boolOr(r.SshEnabled, target.SshEnabled)
			target.PushEnabled = boolOr(r.PushEnabled, target.PushEnabled)

			found = true
			nodeId = target.Id
			node = *target
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if !found {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		go coordinator.CheckSshStatus(nodeId)
		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, node)
	}
}

// NodesUpdatePartialBatch updates the usage of multiple nodes.
func NodesUpdatePartialBatch(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request NodesUpdatePartialRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		err := db.Write(func(d *data.Data) {
			for _, node := range d.Nodes {
				if request.Usage != nil {
					node.Usage = *request.Usage
					node.UsageBytes = util.GB2Bytes(*request.Usage)
				}
			}
		})
		if err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// NodesUpdateToggles updates the sync flags (ssh/push/pull) of a single node.
func NodesUpdateToggles(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesTogglesRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}

		var node data.Node
		var nodeId string
		sshTurnedOn := false
		found := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			for _, n := range d.Nodes {
				if n.Id != c.Param("id") {
					continue
				}
				if r.SshEnabled != nil && *r.SshEnabled != n.SshEnabled {
					n.SshEnabled = *r.SshEnabled
					n.SshStatus = data.NodeStatusProcessing
					sshTurnedOn = *r.SshEnabled
				}
				if r.PushEnabled != nil && *r.PushEnabled != n.PushEnabled {
					n.PushEnabled = *r.PushEnabled
					n.PushStatus = data.NodeStatusProcessing
				}
				found = true
				nodeId = n.Id
				node = *n
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if !found {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		if sshTurnedOn {
			go coordinator.CheckSshStatus(nodeId)
		}
		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, node)
	}
}

// NodesDelete deletes a node.
func NodesDelete(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		deleted := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			for i, s := range d.Nodes {
				if s.Id == c.Param("id") {
					d.Nodes = append(d.Nodes[:i], d.Nodes[i+1:]...)
					deleted = true
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if deleted {
			go coordinator.UpdateConfigs()
		}

		return c.NoContent(http.StatusNoContent)
	}
}
