package pages

import (
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"net/http"
	"os"
	"path/filepath"
)

func Profile(config *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {
		content, err := os.ReadFile(filepath.Join(config.Env.AppDirectory, "web/profile.html"))
		if err != nil {
			return err
		}

		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		return c.HTML(http.StatusOK, string(content))
	}
}
