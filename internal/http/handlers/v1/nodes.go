package v1

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"net/http"
	"strconv"
)

type NodesStoreRequest struct {
	Host      string `json:"host" validate:"required,max=64"`
	HttpToken string `json:"http_token" validate:"required"`
	HttpPort  int    `json:"http_port" validate:"required,min=1,max=65536"`
}

type NodesUpdateRequest struct {
	NodesStoreRequest
	Id int `json:"id"`
}

func NodesIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Servers)
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

		if len(d.Data.Servers) > 5 {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": fmt.Sprintf("Cannot add more servers!"),
			})
		}

		node := &database.Server{}
		node.Id = d.GenerateServerId()
		node.Status = database.ServerStatusProcessing
		node.Traffic = 0
		node.HttpToken = r.HttpToken
		node.Host = r.Host
		node.HttpPort = r.HttpPort

		d.Data.Servers = append(d.Data.Servers, node)
		d.Save()

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

		var node *database.Server
		for _, n := range d.Data.Servers {
			if n.Id == r.Id {
				node = n
			}
		}
		if node == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		node.Host = r.Host
		node.HttpToken = r.HttpToken
		node.HttpPort = r.HttpPort
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, node)

	}
}

func NodesDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the id parameter.",
			})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		for i, s := range d.Data.Servers {
			if s.Id == id {
				d.Data.Servers = append(d.Data.Servers[:i], d.Data.Servers[i+1:]...)
				d.Save()
				go coordinator.SyncConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}