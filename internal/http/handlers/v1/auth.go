package v1

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/pkg/enigma"
	"net/http"
	"time"
)

type SignInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SignIn(d *database.Database, e *enigma.Enigma) echo.HandlerFunc {
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

		if r.Username == "admin" && r.Password == d.Data.Settings.AdminPassword {
			return c.JSON(http.StatusOK, map[string]string{
				"token": d.Data.Settings.AdminPassword,
			})
		}

		plain := fmt.Sprintf("%s:%d", d.Data.Settings.Host, 0)
		if r.Username == "admin" && e.Verify([]byte(plain), []byte(r.Password)) {
			return c.JSON(http.StatusOK, map[string]string{
				"token": d.Data.Settings.AdminPassword,
			})
		}

		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "Unauthorized.",
		})
	}
}
