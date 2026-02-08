package v1

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

		if !util.PortsDistinct([]int{r.SsRelayPort, r.SsReversePort, r.SsDirectPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Shadowsocks ports must be the distinct.",
			})
		}

		current := d.Data().Settings
		if r.SsRelayPort > 0 && r.SsRelayPort != current.SsRelayPort && !util.PortFree(r.SsRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsRelayPort),
			})
		}
		if r.SsReversePort > 0 && r.SsReversePort != current.SsReversePort && !util.PortFree(r.SsReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsReversePort),
			})
		}
		if r.SsDirectPort > 0 && r.SsDirectPort != current.SsDirectPort && !util.PortFree(r.SsDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.SsDirectPort),
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
