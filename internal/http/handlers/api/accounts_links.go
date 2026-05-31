package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
)

// AccountLinksRenew renews the proxy links of an account.
func AccountLinksRenew(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		accountId := c.Param("accountId")
		if accountId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Account is required.",
			})
		}

		var account data.Account
		found := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			a := d.FindAccountById(accountId)
			if a == nil {
				return false, nil
			}
			a.ProxyId = util.Uuid()
			account = *a
			found = true
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if !found {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Account not found.",
			})
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}
