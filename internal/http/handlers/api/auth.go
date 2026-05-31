package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
)

const sleepAfterSignInRequest = 3 * time.Second

type SignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SignIn signs in an admin account and returns a token.
func SignIn(db *data.Store) echo.HandlerFunc {
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

		var password string
		db.Read(func(d *data.Data) {
			password = d.MainSettings.AdminPassword
		})

		if r.Username == "admin" && r.Password == password {
			return c.JSON(http.StatusOK, map[string]string{
				"token": password,
			})
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "Unauthorized.",
		})
	}
}
