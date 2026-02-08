package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

type ProfileResponse struct {
	User      data.User `json:"user"`
	SsDirect  string    `json:"ss_direct"`
	SsRelay   string    `json:"ss_relay"`
	SsReverse string    `json:"ss_reverse"`
	SsRemote  string    `json:"ss_remote"`
}

func ProfileShow(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *data.User
		for _, u := range db.Data().Users {
			if u.VlessId == c.QueryParam("u") {
				user = u
			}
		}
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		r := ProfileResponse{User: *user}
		r.User.Usage = r.User.Usage * db.Data().MainSettings.TrafficRatio
		r.User.Quota = r.User.Quota * db.Data().MainSettings.TrafficRatio

		return c.JSON(http.StatusOK, r)
	}
}

func ProfileRegenerate(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *data.User
		for _, u := range db.Data().Users {
			if u.VlessId == c.QueryParam("u") {
				user = u
			}
		}
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
