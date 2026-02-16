package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

const sleepAfterSignInRequest = 3 * time.Second

type SignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SignIn signs in a user and returns a token.
func SignIn(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			time.Sleep(sleepAfterSignInRequest)
		}()

		var r SignInRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}

		if r.Username == "admin" && r.Password == db.Data().MainSettings.AdminPassword {
			return c.JSON(http.StatusOK, map[string]string{
				"token": db.Data().MainSettings.AdminPassword,
			})
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "Unauthorized.",
		})
	}
}
