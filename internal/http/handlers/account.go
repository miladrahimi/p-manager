package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
)

// Account returns the account page for an account.
func Account(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		accountId := c.Param("accountId")
		if accountId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Account is required.",
			})
		}

		found := false
		db.Read(func(d *data.Data) {
			for _, u := range d.Accounts {
				if u.Id == accountId {
					found = true
					return
				}
			}
		})

		if !found {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Account not found.",
			})
		}

		content, err := os.ReadFile(filepath.Join("web", "account.html"))
		if err != nil {
			return err
		}
		return c.HTML(http.StatusOK, string(content))
	}
}
