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
)

// XraySettingsShow returns the xray settings.
func XraySettingsShow(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var settings data.XraySettings
		db.Read(func(d *data.Data) {
			settings = *d.XraySettings
		})
		return c.JSON(http.StatusOK, &settings)
	}
}

// XraySettingsUpdate updates the xray settings.
func XraySettingsUpdate(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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
			r.DirectRrPort,
			r.RemoteRrPort,
			r.RelayRr2RrManagerPort,
			r.RelayRr2RrNodePort,
			r.RelayRr2SshPort,
		}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Ports must be the distinct.",
			})
		}

		var d data.XraySettings
		db.Read(func(s *data.Data) {
			d = *s.XraySettings
		})
		if r.DirectRrPort > 0 && r.DirectRrPort != d.DirectRrPort && !util.PortFree(r.DirectRrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.DirectRrPort),
			})
		}
		if r.RelayRr2RrManagerPort > 0 && r.RelayRr2RrManagerPort != d.RelayRr2RrManagerPort &&
			!util.PortFree(r.RelayRr2RrManagerPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.RelayRr2RrManagerPort),
			})
		}
		if r.RelayRr2SshPort > 0 && r.RelayRr2SshPort != d.RelayRr2SshPort && !util.PortFree(r.RelayRr2SshPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", r.RelayRr2SshPort),
			})
		}

		if err := db.Write(func(s *data.Data) {
			s.XraySettings = &r
		}); err != nil {
			return errors.WithStack(err)
		}

		coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, r)
	}
}
