package v1

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/utils"
	"net/http"
)

func SettingsGeneralMainShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Settings)
	}
}

func SettingsGeneralMainUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var s database.Settings
		if err := c.Bind(&s); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(s); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if !utils.PortsUnique([]int{s.SsRelayPort, s.SsReversePort, s.SsDirectPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Proxy ports must be the unique.",
			})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		ds := d.Data.Settings
		if s.SsRelayPort > 0 && s.SsRelayPort != ds.SsRelayPort && !utils.PortFree(s.SsRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsRelayPort),
			})
		}
		if s.SsReversePort > 0 && s.SsReversePort != ds.SsReversePort && !utils.PortFree(s.SsReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsReversePort),
			})
		}
		if s.SsDirectPort > 0 && s.SsDirectPort != ds.SsDirectPort && !utils.PortFree(s.SsDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsDirectPort),
			})
		}

		d.Data.Settings = &s
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, s)
	}
}

func SettingsGeneralRestartXray(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.SyncConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsGeneralUsersDisabledDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		var newUsers []*database.User
		for _, u := range d.Data.Users {
			if u.Enabled {
				newUsers = append(newUsers, u)
			}
		}

		d.Data.Users = newUsers
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsGeneralUsersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		d.Data.Users = []*database.User{}
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}
