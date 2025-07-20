package v1

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/utils"
	"net/http"
	"strconv"
)

type NodeResponse struct {
	database.Node
	PullCommand string `json:"pull_command"`
}

type NodesStoreRequest struct {
	Host      string `json:"host" validate:"required,max=64"`
	HttpToken string `json:"http_token" validate:"required"`
	HttpPort  int    `json:"http_port" validate:"required,min=1,max=65536"`
}

type NodesUpdateRequest struct {
	NodesStoreRequest
}

type NodesUpdatePartialRequest struct {
	Usage *float64 `json:"usage"`
}

func NodesIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := d.Content.Settings.AdminPassword

		var response = make([]NodeResponse, 0, len(d.Content.Nodes))
		for _, node := range d.Content.Nodes {
			cmd := fmt.Sprintf("make set-manager URL=\"BASE_URL/v1/nodes/%d\" TOKEN=\"%s\"", node.Id, token)
			response = append(response, NodeResponse{
				Node:        *node,
				PullCommand: cmd,
			})
		}

		return c.JSON(http.StatusOK, response)
	}
}

func NodesStore(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		if len(d.Content.Nodes) > 5 {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": fmt.Sprintf("Cannot add more nodes!"),
			})
		}

		var node *database.Node
		for _, n := range d.Content.Nodes {
			if n.Host == r.Host && n.HttpPort == r.HttpPort {
				node = n
				node.HttpToken = r.HttpToken
			}
		}
		if node == nil {
			node = &database.Node{}
			node.Id = d.GenerateNodeId()
			node.HttpToken = r.HttpToken
			node.Host = r.Host
			node.HttpPort = r.HttpPort

			d.Content.Nodes = append(d.Content.Nodes, node)
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusCreated, node)
	}
}

func NodesUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		var node *database.Node
		for _, n := range d.Content.Nodes {
			if strconv.Itoa(n.Id) == c.Param("id") {
				node = n
			}
		}
		if node == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		node.Host = r.Host
		node.HttpToken = r.HttpToken
		node.HttpPort = r.HttpPort

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, node)

	}
}

func NodesUpdatePartialBatch(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		for _, node := range d.Content.Nodes {
			if request.Usage != nil {
				node.Usage = *request.Usage
				node.UsageBytes = utils.GB2Bytes(*request.Usage)
			}
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func NodesDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		for i, s := range d.Content.Nodes {
			if strconv.Itoa(s.Id) == c.Param("id") {
				d.Content.Nodes = append(d.Content.Nodes[:i], d.Content.Nodes[i+1:]...)
				if err := d.Save(); err != nil {
					return errors.WithStack(err)
				}
				go coordinator.SyncConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}
