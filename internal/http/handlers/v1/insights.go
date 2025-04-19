package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/database"
	"net/http"
)

func InsightsIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			TotalUsers  int `json:"total_users"`
			ActiveUsers int `json:"active_users"`
		}{
			TotalUsers:  len(d.Content.Users),
			ActiveUsers: d.CountActiveUsers(),
		})
	}
}
