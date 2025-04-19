package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/database"
	"net/http"
	"time"
)

func StatsIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Content.Stats)
	}
}

func StatsTotalUsageReset(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		d.Content.Stats.TotalUsage = 0
		d.Content.Stats.TotalUsageResetAt = time.Now().UnixMilli()
		d.Save()

		return c.JSON(http.StatusOK, d.Content.Stats)
	}
}
