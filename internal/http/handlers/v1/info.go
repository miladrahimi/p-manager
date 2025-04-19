package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/licensor"
	"net/http"
)

func InfoIndex(l *licensor.Licensor) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			AppName     string `json:"app_name"`
			AppVersion  string `json:"app_version"`
			AppLicensed bool   `json:"app_licensed"`
			Core        string `json:"core"`
		}{
			AppName:     config.AppName,
			AppVersion:  config.AppVersion,
			AppLicensed: l.Licensed(),
			Core:        config.CoreVersion,
		})
	}
}
