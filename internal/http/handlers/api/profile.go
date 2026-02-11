package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

type ProfileResponse struct {
	User    data.User         `json:"user"`
	Proxies map[string]string `json:"proxies"`
}

func ProfileShow(composer *composer.Composer, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		userId := c.QueryParam("u")
		if userId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "User is required.",
			})
		}

		u := db.Data().FindUserById(userId)
		if u == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "User not found.",
			})
		}

		d := db.Data()
		TrafficRatio := d.MainSettings.TrafficRatio

		r := ProfileResponse{User: *u, Proxies: make(map[string]string)}
		r.User.Usage = r.User.Usage * TrafficRatio
		r.User.Quota = r.User.Quota * TrafficRatio

		r.Proxies = composer.UserLinks(u)

		return c.JSON(http.StatusOK, r)
	}
}

func ProfileLinksRenew(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		userId := c.QueryParam("u")
		if userId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "User is required.",
			})
		}

		user := db.Data().FindUserById(userId)
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "User not found.",
			})
		}

		user.VlessId = util.Uuid()

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
