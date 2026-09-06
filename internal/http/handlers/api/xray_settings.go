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
	nodeUtil "github.com/miladrahimi/p-node/pkg/util"
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
			r.ReverseRrManagerPort,
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
		// A manager port that changes must be free on this host.
		for _, p := range [][2]int{
			{r.DirectRrPort, d.DirectRrPort},
			{r.RelayRr2RrManagerPort, d.RelayRr2RrManagerPort},
			{r.RelayRr2SshPort, d.RelayRr2SshPort},
			{r.ReverseRrManagerPort, d.ReverseRrManagerPort},
		} {
			if p[0] > 0 && p[0] != p[1] && !nodeUtil.PortFree(p[0]) {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": fmt.Sprintf("Port %d is already in use.", p[0]),
				})
			}
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
