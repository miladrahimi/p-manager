package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
)

func DetailsShow() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			AppName    string `json:"app_name"`
			AppVersion string `json:"app_version"`
			Core       string `json:"core"`
		}{
			AppName:    config.AppName,
			AppVersion: config.AppVersion,
			Core:       config.CoreVersion,
		})
	}
}
