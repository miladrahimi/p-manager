package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
)

// XrayRestart restarts the xray service.
func XrayRestart(coordinator *coordinator.Coordinator) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		go coordinator.UpdateConfigs()
		return ctx.NoContent(http.StatusNoContent)
	}
}
