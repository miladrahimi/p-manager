package v1

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

type SignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SignIn(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			time.Sleep(time.Second * time.Duration(2))
		}()

		var r SignInRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}

		if r.Username == "admin" && r.Password == d.Data().Settings.AdminPassword {
			return c.JSON(http.StatusOK, map[string]string{
				"token": d.Data().Settings.AdminPassword,
			})
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "Unauthorized.",
		})
	}
}
