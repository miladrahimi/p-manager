package handlers

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
)

func Profile() echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.QueryParam("c") != "" {
			return c.Redirect(http.StatusMovedPermanently, "/profile?u="+c.QueryParam("c"))
		}

		content, err := os.ReadFile("web/profile.html")
		if err != nil {
			return err
		}

		return c.HTML(http.StatusOK, string(content))
	}
}
