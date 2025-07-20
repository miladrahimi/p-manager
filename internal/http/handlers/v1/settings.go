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
)

func SettingsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Content.Settings)
	}
}

func SettingsUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r database.Settings
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

		if !utils.PortsDistinct([]int{r.SsRelayPort, r.SsReversePort, r.SsDirectPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Shadowsocks ports must be the distinct.",
			})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		current := d.Content.Settings
		if r.SsRelayPort > 0 && r.SsRelayPort != current.SsRelayPort && !utils.PortFree(r.SsRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsRelayPort),
			})
		}
		if r.SsReversePort > 0 && r.SsReversePort != current.SsReversePort && !utils.PortFree(r.SsReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsReversePort),
			})
		}
		if r.SsDirectPort > 0 && r.SsDirectPort != current.SsDirectPort && !utils.PortFree(r.SsDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsDirectPort),
			})
		}

		d.Content.Settings = &r

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, r)
	}
}

func SettingsXrayRestart(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.SyncConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}
