package api

import (
	"encoding/base64"
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

// SubscriptionShow returns the proxy subscription for a user proxy ID.
func SubscriptionShow(composer *composer.Composer, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		proxyId := c.Param("proxyId")
		if proxyId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Proxy ID is required.",
			})
		}

		u := db.Data().FindUserByProxyId(proxyId)
		if u == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "User not found.",
			})
		}

		links := composer.UserLinks(u)
		var items []string
		for _, link := range links {
			items = append(items, link)
		}
		slices.Sort(items)
		payload := base64.StdEncoding.EncodeToString([]byte(strings.Join(items, "\n")))

		c.Response().Header().Set("Content-Type", "text/plain")
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		return c.String(http.StatusOK, payload)
	}
}
