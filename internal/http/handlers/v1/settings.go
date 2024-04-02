package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"net/http"
	"time"
)

func SettingsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Settings)
	}
}

func SettingsUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var s database.Settings
		if err := c.Bind(&s); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(s); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if !utils.PortsUnique([]int{s.SsRelayPort, s.SsReversePort, s.SsDirectPort}) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Proxy ports must be the unique.",
			})
		}

		ds := d.Data.Settings
		if s.SsRelayPort > 0 && s.SsRelayPort != ds.SsRelayPort && !utils.PortFree(s.SsRelayPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsRelayPort),
			})
		}
		if s.SsReversePort > 0 && s.SsReversePort != ds.SsReversePort && !utils.PortFree(s.SsReversePort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsReversePort),
			})
		}
		if s.SsDirectPort > 0 && s.SsDirectPort != ds.SsDirectPort && !utils.PortFree(s.SsDirectPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Port %d is already in use.", s.SsDirectPort),
			})
		}

		d.Data.Settings = &s
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, s)
	}
}

func SettingsRestartXray(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.SyncConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsInsightsShow(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			Stats            database.Stats `json:"stats"`
			UsersCount       int            `json:"users_count"`
			ActiveUsersCount int            `json:"active_users_count"`
			AppName          string         `json:"app_name"`
			AppVersion       string         `json:"app_version"`
			AppLicensed      bool           `json:"app_licensed"`
			Core             string         `json:"core"`
		}{
			Stats:            *d.Data.Stats,
			UsersCount:       len(d.Data.Users),
			ActiveUsersCount: d.CountActiveUsers(),
			AppName:          config.AppName,
			AppVersion:       config.AppVersion,
			AppLicensed:      coordinator.Licensed(),
			Core:             config.CoreDetails,
		})
	}
}

func SettingsStatsZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Stats.Traffic = 0
		d.Data.Stats.UpdatedAt = time.Now().UnixMilli()
		d.Save()
		return c.JSON(http.StatusOK, d.Data.Stats)
	}
}

func SettingsServersZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, s := range d.Data.Servers {
			s.Traffic = 0
		}
		d.Save()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsUsersZero(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, u := range d.Data.Users {
			u.Used = 0
			u.UsedBytes = 0
			u.Enabled = true
		}
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsUsersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Users = []*database.User{}
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}
