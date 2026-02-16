package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

type NodeResponse struct {
	data.Node
	PullCommand string `json:"pull_command"`
}

type NodesStoreRequest struct {
	Host      string `json:"host" validate:"required,max=128"`
	Ip        string `json:"ip" validate:"max=128"`
	HttpToken string `json:"http_token" validate:"required"`
	HttpPort  int    `json:"http_port" validate:"required,min=1,max=65535"`
	SshUser   string `json:"ssh_user" validate:"required"`
	SshPort   int    `json:"ssh_port" validate:"required,min=1,max=65535"`
}

type NodesUpdateRequest struct {
	NodesStoreRequest
}

type NodesUpdatePartialRequest struct {
	Usage *float64 `json:"usage"`
}

// NodesIndex returns the list of nodes.
func NodesIndex(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := db.Data().MainSettings.AdminPassword

		var response = make([]NodeResponse, 0, len(db.Data().Nodes))
		for _, node := range db.Data().Nodes {
			cmd := fmt.Sprintf("make set-manager URL=\"BASE_URL/v1/nodes/%s\" TOKEN=\"%s\"", node.Id, token)
			response = append(response, NodeResponse{
				Node:        *node,
				PullCommand: cmd,
			})
		}

		return c.JSON(http.StatusOK, response)
	}
}

// NodesStore stores a new node.
func NodesStore(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesStoreRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if r.Host == "" && r.Ip != "" {
			r.Host = r.Ip
		}
		if err := validator.New().Struct(r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if len(db.Data().Nodes) > 5 {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": fmt.Sprintf("Cannot add more nodes!"),
			})
		}

		var node *data.Node
		sshStatus := data.NodeStatusProcessing
		for _, n := range db.Data().Nodes {
			if n.Host == r.Host && n.HttpPort == r.HttpPort {
				node = n
				node.HttpToken = r.HttpToken
				node.SshUser = r.SshUser
				node.SshPort = r.SshPort
				node.SshStatus = sshStatus
			}
		}
		if node == nil {
			node = data.NewNode(util.Uuid(), r.Host, r.HttpToken, r.HttpPort, r.SshUser, r.SshPort)
			node.SshStatus = sshStatus
			db.Data().Nodes = append(db.Data().Nodes, node)
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.CheckSshStatus(node.Id)
		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, node)
	}
}

// NodesUpdate updates a node.
func NodesUpdate(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesUpdateRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if r.Host == "" && r.Ip != "" {
			r.Host = r.Ip
		}
		if err := validator.New().Struct(r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		var node *data.Node
		for _, n := range db.Data().Nodes {
			if n.Id == c.Param("id") {
				node = n
			}
		}
		if node == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		sshStatus := data.NodeStatusProcessing

		node.Host = r.Host
		node.HttpToken = r.HttpToken
		node.HttpPort = r.HttpPort
		node.SshUser = r.SshUser
		node.SshPort = r.SshPort
		node.SshStatus = sshStatus
		node.SshStatus = sshStatus

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.CheckSshStatus(node.Id)
		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, node)

	}
}

// NodesUpdatePartialBatch updates the usage of multiple nodes.
func NodesUpdatePartialBatch(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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

		for _, node := range db.Data().Nodes {
			if request.Usage != nil {
				node.Usage = *request.Usage
				node.UsageBytes = util.GB2Bytes(*request.Usage)
			}
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// NodesDelete deletes a node.
func NodesDelete(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		for i, s := range db.Data().Nodes {
			if s.Id == c.Param("id") {
				db.Data().Nodes = append(db.Data().Nodes[:i], db.Data().Nodes[i+1:]...)
				if err := db.Save(); err != nil {
					return errors.WithStack(err)
				}
				go coordinator.UpdateConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}
