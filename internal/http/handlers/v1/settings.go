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

func SettingsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Content.Settings)
	}
}

func SettingsUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var input database.Settings
		if err := c.Bind(&input); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(input); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if !utils.PortsUnique([]int{input.SsRelayPort, input.SsReversePort, input.SsDirectPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Proxy ports must be the unique.",
			})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		old := d.Content.Settings
		if input.SsRelayPort > 0 && input.SsRelayPort != old.SsRelayPort && !utils.PortFree(input.SsRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", input.SsRelayPort),
			})
		}
		if input.SsReversePort > 0 && input.SsReversePort != old.SsReversePort && !utils.PortFree(input.SsReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", input.SsReversePort),
			})
		}
		if input.SsDirectPort > 0 && input.SsDirectPort != old.SsDirectPort && !utils.PortFree(input.SsDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", input.SsDirectPort),
			})
		}

		d.Content.Settings = &input
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, input)
	}
}

func SettingsXrayRestart(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.SyncConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}
