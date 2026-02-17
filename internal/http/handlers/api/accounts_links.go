package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

// AccountLinksRenew renews the proxy links of an account.
func AccountLinksRenew(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		accountId := c.Param("accountId")
		if accountId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Account is required.",
			})
		}

		account := db.Data().FindAccountById(accountId)
		if account == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Account not found.",
			})
		}

		account.ProxyId = util.Uuid()

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}
