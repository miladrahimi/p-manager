package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

func InsightsIndex(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		d := db.Data()
		return c.JSON(http.StatusOK, struct {
			TotalUsers  int `json:"total_users"`
			ActiveUsers int `json:"active_users"`
		}{
			TotalUsers:  len(d.Users),
			ActiveUsers: d.CountActiveUsers(),
		})
	}
}
