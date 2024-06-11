package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/licensor"
	"net/http"
	"time"
)

func SettingsInsightsShow(d *database.Database, l *licensor.Licensor) echo.HandlerFunc {
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
			AppLicensed:      l.Licensed(),
			Core:             config.CoreVersion,
		})
	}
}

func SettingsInsightsStatsZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		d.Data.Stats.Traffic = 0
		d.Data.Stats.UpdatedAt = time.Now().UnixMilli()
		d.Save()

		return c.JSON(http.StatusOK, d.Data.Stats)
	}
}

func SettingsInsightsNodesZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		for _, s := range d.Data.Servers {
			s.Traffic = 0
		}
		d.Save()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsInsightsUsersZero(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

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
