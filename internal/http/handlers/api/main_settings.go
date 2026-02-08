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

func SettingsShow(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data().Settings)
	}
}

func SettingsUpdate(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r data.Settings
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

		if !util.PortsDistinct([]int{r.VtnVtrRelayPort, r.VtrDirectPort, r.VtnVtrReversePort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Shadowsocks ports must be the distinct.",
			})
		}

		current := d.Data().Xray
		if r.VtnVtrRelayPort > 0 && r.VtnVtrRelayPort != current.VtnVtrRelayPort && !util.PortFree(r.VtnVtrRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VtnVtrRelayPort),
			})
		}
		if r.VtrDirectPort > 0 && r.VtrDirectPort != current.VtrDirectPort && !util.PortFree(r.VtrDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VtrDirectPort),
			})
		}
		if r.VtnVtrReversePort > 0 && r.VtnVtrReversePort != current.VtnVtrReversePort && !util.PortFree(r.VtnVtrReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VtnVtrReversePort),
			})
		}

		d.Data().Settings = &r

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, r)
	}
}

func SettingsXrayRestart(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.UpdateConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}
