package v1

import (
	"encoding/base64"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"net/http"
)

type ProfileResponse struct {
	User      database.User `json:"user"`
	SsDirect  string        `json:"ss_direct"`
	SsRelay   string        `json:"ss_relay"`
	SsReverse string        `json:"ss_reverse"`
	SsRemote  string        `json:"ss_remote"`
}

func ProfileShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *database.User
		for _, u := range d.Content.Users {
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
		r.User.Usage = r.User.Usage * d.Content.Settings.TrafficRatio
		r.User.Quota = r.User.Quota * d.Content.Settings.TrafficRatio

		s := d.Content.Settings
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

func ProfileRegenerate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		var user *database.User
		for _, u := range d.Content.Users {
			if u.Identity == c.QueryParam("u") {
				user = u
			}
		}
		if user == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		user.ShadowsocksPassword = d.GenerateUserPassword()

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
