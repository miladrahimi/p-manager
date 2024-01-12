package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"net/http"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/coordinator"
	"shadowsocks-manager/internal/database"
	"strconv"
)

type ServersStoreRequest struct {
	Host     string `json:"host" validate:"required,max=64"`
	Port     int    `json:"port" validate:"required,min=1,max=65536"`
	Password string `json:"password" validate:"required"`
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
		var request ServersStoreRequest
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

		for _, s := range d.Data.Servers {
			if s.Host == request.Host && s.Port == request.Port && s.Password == request.Password {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The server is already exist.",
				})
			}
		}

		server := &database.Server{}
		server.Id = d.GenerateServerId()
		server.Status = database.ServerStatusProcessing
		server.Method = config.ShadowsocksMethod
		server.Password = request.Password
		server.Host = request.Host
		server.Port = request.Port

		d.Data.Servers = append(d.Data.Servers, server)
		d.Save()

		go coordinator.SyncServersAndStatuses()

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

		if server != nil {
			server.Host = request.Host
			server.Port = request.Port
			server.Password = request.Password
			d.Save()
			go coordinator.SyncServersAndStatuses()
			return c.JSON(http.StatusOK, server)
		}

		return c.NoContent(http.StatusNotFound)
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

		for i, s := range d.Data.Servers {
			if s.Id == id {
				d.Data.Servers = append(d.Data.Servers[:i], d.Data.Servers[i+1:]...)
				d.Save()
				go coordinator.SyncServers()
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}
