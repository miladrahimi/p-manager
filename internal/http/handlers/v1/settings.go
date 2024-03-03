package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
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
		var settings database.Settings
		if err := c.Bind(&settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		d.Data.Settings = &settings
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, settings)
	}
}

func SettingsRestartXray(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(c echo.Context) error {
		go coordinator.SyncConfigs()
		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsStatsShow(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		UsersCount := len(d.Data.Users)
		ActiveUsersCount := UsersCount
		for _, u := range d.Data.Users {
			if !u.Enabled {
				ActiveUsersCount--
			}
		}
		return c.JSON(http.StatusOK, struct {
			*database.Stats
			UsersCount       int    `json:"users_count"`
			ActiveUsersCount int    `json:"active_users_count"`
			AppVersion       string `json:"app_version"`
			Licensed         bool   `json:"licensed"`
		}{
			Stats:            d.Data.Stats,
			UsersCount:       UsersCount,
			ActiveUsersCount: ActiveUsersCount,
			AppVersion:       config.AppVersion,
			Licensed:         coordinator.Licensed(),
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
