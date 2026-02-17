package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

// InsightsIndex returns the insights index.
func InsightsIndex(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		d := db.Data()
		return c.JSON(http.StatusOK, struct {
			TotalAccounts  int `json:"total_accounts"`
			ActiveAccounts int `json:"active_accounts"`
		}{
			TotalAccounts:  len(d.Accounts),
			ActiveAccounts: d.CountActiveAccounts(),
		})
	}
}
