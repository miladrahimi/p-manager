package v1

import (
	"encoding/base64"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"net/http"
)

type ProfileResponse struct {
	User      database.User `json:"user"`
	SsReverse string        `json:"ss_reverse"`
	SsRelay   string        `json:"ss_relay"`
}

func ProfileShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *database.User
		for _, u := range d.Data.Users {
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
		r.User.Used = r.User.Used * d.Data.Settings.TrafficRatio
		r.User.Quota = r.User.Quota * d.Data.Settings.TrafficRatio

		s := d.Data.Settings
		auth := base64.StdEncoding.EncodeToString([]byte(user.ShadowsocksMethod + ":" + user.ShadowsocksPassword))

		if s.SsReversePort > 0 {
			r.SsReverse = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsReversePort, "reverse")
		}

		if s.SsRelayPort > 0 {
			r.SsRelay = fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.SsRelayPort, "relay")
		}

		return c.JSON(http.StatusOK, r)
	}
}

func ProfileReset(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user *database.User
		for _, u := range d.Data.Users {
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
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
