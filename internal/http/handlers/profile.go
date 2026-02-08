package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

func Profile(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		userId := c.QueryParams().Get("u")
		for _, u := range d.Data().Users {
			if u.Identity == userId {
				content, err := os.ReadFile(filepath.Join("web", "profile.html"))
				if err != nil {
					return err
				}
				return c.HTML(http.StatusOK, string(content))
			}
		}

		content, err := os.ReadFile(filepath.Join("web", "profile-404.html"))
		if err != nil {
			return err
		}
		return c.HTML(http.StatusOK, string(content))
	}
}
