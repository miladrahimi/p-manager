package v1

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"shadowsocks-manager/internal/coordinator"
	"shadowsocks-manager/internal/database"
	"time"
)

func StatsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Stats)
	}
}

func StatsZeroServers(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Stats.Inbound = 0
		d.Data.Stats.Outbound = 0
		d.Data.Stats.Freedom = 0
		d.Data.Stats.UpdatedAt = time.Now().UnixMilli()
		d.Save()

		return c.JSON(http.StatusOK, d.Data.Stats)
	}
}

func StatsZeroUsers(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, u := range d.Data.Users {
			u.Used = 0
			u.UsedBytes = 0
			u.Enabled = true
		}
		d.Save()

		return c.NoContent(http.StatusNoContent)
	}
}

func StatsDeleteAllUsers(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Users = []*database.User{}
		d.Save()

		go coordinator.SyncUsers()

		return c.NoContent(http.StatusNoContent)
	}
}
