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

		if !util.PortsDistinct([]int{r.VrrvDirectPort, r.VrrvRemotePort, r.Vrrv2VrrvPort, r.Vrrv2SshPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Ports must be the distinct.",
			})
		}

		current := db.Data().XraySettings
		if r.VrrvDirectPort > 0 && r.VrrvDirectPort != current.VrrvDirectPort && !util.PortFree(r.VrrvDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VrrvDirectPort),
			})
		}
		if r.VrrvRemotePort > 0 && r.VrrvRemotePort != current.VrrvRemotePort && !util.PortFree(r.VrrvRemotePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.VrrvRemotePort),
			})
		}
		if r.Vrrv2VrrvPort > 0 && r.Vrrv2VrrvPort != current.Vrrv2VrrvPort && !util.PortFree(r.Vrrv2VrrvPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.Vrrv2VrrvPort),
			})
		}
		if r.Vrrv2SshPort > 0 && r.Vrrv2SshPort != current.Vrrv2SshPort && !util.PortFree(r.Vrrv2SshPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.Vrrv2SshPort),
			})
		}

		db.Data().XraySettings = &r
		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, r)
	}
}
