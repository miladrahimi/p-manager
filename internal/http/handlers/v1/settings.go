package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"net/http"
	"shadowsocks-manager/internal/coordinator"
	"shadowsocks-manager/internal/database"
	"shadowsocks-manager/internal/utils"
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

		if settings.ShadowsocksPort != d.Data.Settings.ShadowsocksPort && !utils.PortFree(settings.ShadowsocksPort) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("The shadowsocks port is not free: %v", settings.ShadowsocksPort),
			})
		}

		d.Data.Settings = &settings
		d.Save()

		go coordinator.SyncSettings()

		return c.JSON(http.StatusOK, settings)
	}
}
