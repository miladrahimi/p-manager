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
			r.DirectRrPort,
			r.RemoteRrPort,
			r.RelayRr2RrPort,
			r.RelayRr2SshPort,
		}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Ports must be the distinct.",
			})
		}

		current := db.Data().XraySettings
		if r.DirectRrPort > 0 && r.DirectRrPort != current.DirectRrPort && !util.PortFree(r.DirectRrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.DirectRrPort),
			})
		}
		if r.RemoteRrPort > 0 && r.RemoteRrPort != current.RemoteRrPort && !util.PortFree(r.RemoteRrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.RemoteRrPort),
			})
		}
		if r.RelayRr2RrPort > 0 && r.RelayRr2RrPort != current.RelayRr2RrPort && !util.PortFree(r.RelayRr2RrPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.RelayRr2RrPort),
			})
		}
		if r.RelayRr2SshPort > 0 && r.RelayRr2SshPort != current.RelayRr2SshPort && !util.PortFree(r.RelayRr2SshPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("ServerPort %d is already in use.", r.RelayRr2SshPort),
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
