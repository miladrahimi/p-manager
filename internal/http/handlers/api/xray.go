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

func XraySettingsShow(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, db.Data().XraySettings)
	}
}

func XraySettingsUpdate(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r data.XraySettings
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

		if !util.PortsDistinct([]int{r.VtrDirectPort, r.VtrRemotePort, r.Vt2VtrPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Ports must be the distinct.",
			})
		}

		current := db.Data().XraySettings
		if r.VtrDirectPort > 0 && r.VtrDirectPort != current.VtrDirectPort && !util.PortFree(r.VtrDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VtrDirectPort),
			})
		}
		if r.VtrRemotePort > 0 && r.VtrRemotePort != current.VtrRemotePort && !util.PortFree(r.VtrRemotePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VtrRemotePort),
			})
		}
		if r.Vt2VtrPort > 0 && r.Vt2VtrPort != current.Vt2VtrPort && !util.PortFree(r.Vt2VtrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.Vt2VtrPort),
			})
		}

		db.Data().XraySettings = &r
		if err := db.Save(); err != nil {
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
