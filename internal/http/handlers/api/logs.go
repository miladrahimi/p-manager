package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer"
)

// LogsLabels returns the id -> label map the log viewer uses to annotate
// account and node identifiers in log lines.
func LogsLabels(composer *composer.Composer) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, composer.LogLabels())
	}
}
