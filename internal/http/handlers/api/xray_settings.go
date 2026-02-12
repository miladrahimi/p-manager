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

		if !util.PortsDistinct([]int{
			r.RrDirectPort,
			r.RrRemotePort,
			r.Rr2RrPort,
			r.Rr2SshPort,
		}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Ports must be the distinct.",
			})
		}

		current := db.Data().XraySettings
		if r.RrDirectPort > 0 && r.RrDirectPort != current.RrDirectPort && !util.PortFree(r.RrDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.RrDirectPort),
			})
		}
		if r.RrRemotePort > 0 && r.RrRemotePort != current.RrRemotePort && !util.PortFree(r.RrRemotePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.RrRemotePort),
			})
		}
		if r.Rr2RrPort > 0 && r.Rr2RrPort != current.Rr2RrPort && !util.PortFree(r.Rr2RrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.Rr2RrPort),
			})
		}
		if r.Rr2SshPort > 0 && r.Rr2SshPort != current.Rr2SshPort && !util.PortFree(r.Rr2SshPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.Rr2SshPort),
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
