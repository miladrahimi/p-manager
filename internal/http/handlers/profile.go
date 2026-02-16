package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

// Profile returns the profile page for a user.
func Profile(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		userId := c.QueryParams().Get("u")
		for _, u := range db.Data().Users {
			if u.Id == userId {
				content, err := os.ReadFile(filepath.Join("web", "profile.html"))
				if err != nil {
					return err
				}
				return c.HTML(http.StatusOK, string(content))
			}
		}

		return c.JSON(http.StatusNotFound, map[string]string{
			"message": "User not found.",
		})
	}
}
