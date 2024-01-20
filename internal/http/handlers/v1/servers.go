package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"net/http"
	"strconv"
)

type ServersStoreRequest struct {
	Host         string `json:"host" validate:"required,max=64"`
	HttpToken    string `json:"http_token" validate:"required"`
	HttpPort     int    `json:"http_port" validate:"required,min=1,max=65536"`
	SsRemotePort int    `json:"ss_remote_port" validate:"required,min=1,max=65536"`
	SsLocalPort  int    `json:"ss_local_port" validate:"min=0,max=65536"`
}

type ServersUpdateRequest struct {
	ServersStoreRequest
	Id int `json:"id"`
}

func ServersIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Servers)
	}
}

func ServersStore(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r ServersStoreRequest
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

		server := &database.Server{}
		server.Id = d.GenerateServerId()
		server.Status = database.ServerStatusProcessing
		server.Traffic = 0
		server.HttpToken = r.HttpToken
		server.Host = r.Host
		server.HttpPort = r.HttpPort
		server.SsLocalPort = r.SsLocalPort
		server.SsRemotePort = r.SsRemotePort

		d.Data.Servers = append(d.Data.Servers, server)
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusCreated, server)
	}
}

func ServersUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request ServersUpdateRequest
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

		var server *database.Server
		for _, s := range d.Data.Servers {
			if s.Id == request.Id {
				server = s
			}
		}
		if server == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Not found."})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		server.Host = request.Host
		server.HttpToken = request.HttpToken
		server.HttpPort = request.HttpPort
		server.SsRemotePort = request.SsRemotePort
		server.SsLocalPort = request.SsLocalPort
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, server)

	}
}

func ServersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}
