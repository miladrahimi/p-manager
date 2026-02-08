package v1

import (
	"encoding/base64"
	"fmt"
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

func ProfileShow(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *data.User
		for _, u := range d.Data().Users {
			if u.Identity == c.QueryParam("u") {
				user = u
			}
		}
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		r := ProfileResponse{User: *user}
		r.User.Usage = r.User.Usage * d.Data().Settings.TrafficRatio
		r.User.Quota = r.User.Quota * d.Data().Settings.TrafficRatio

		s := d.Data().Settings
		auth := base64.StdEncoding.EncodeToString([]byte(user.ShadowsocksMethod + ":" + user.ShadowsocksPassword))

		if s.SsReversePort > 0 {
			r.SsReverse = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsReversePort, "reverse")
		}

		if s.SsRelayPort > 0 {
			r.SsRelay = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsRelayPort, "relay")
		}

		if s.SsDirectPort > 0 {
			r.SsDirect = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsDirectPort, "direct")
		}

		if s.SsRemotePort > 0 {
			r.SsRemote = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsRemotePort, "remote")
		}

		return c.JSON(http.StatusOK, r)
	}
}

func ProfileRegenerate(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *data.User
		for _, u := range d.Data().Users {
			if u.Identity == c.QueryParam("u") {
				user = u
			}
		}
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		user.ShadowsocksPassword = d.Data().GenerateUserPassword()

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
