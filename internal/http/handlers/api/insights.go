package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
)

// InsightsIndex returns the insights index.
func InsightsIndex(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var totalAccounts, activeAccounts int
		db.Read(func(d *data.Data) {
			totalAccounts = len(d.Accounts)
			activeAccounts = d.CountActiveAccounts()
		})
		return c.JSON(http.StatusOK, struct {
			TotalAccounts  int `json:"total_accounts"`
			ActiveAccounts int `json:"active_accounts"`
		}{
			TotalAccounts:  totalAccounts,
			ActiveAccounts: activeAccounts,
		})
	}
}
