package v1

import (
	"encoding/base64"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"net/http"
)

type ProfileResponse struct {
	User             database.User `json:"user"`
	ShadowsocksLinks []string      `json:"shadowsocks_links"`
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

		r := ProfileResponse{User: *user, ShadowsocksLinks: []string{}}
		r.User.Used = r.User.Used * d.Data.Settings.TrafficRatio
		r.User.Quota = int(float64(r.User.Quota) * d.Data.Settings.TrafficRatio)

		s := d.Data.Settings
		auth := base64.StdEncoding.EncodeToString([]byte(user.ShadowsocksMethod + ":" + user.ShadowsocksPassword))

		for i, server := range d.Data.Servers {
			var link string
			var n = i + 1
			if server.SsLocalPort > 0 {
				link = fmt.Sprintf("ss://%s@%s:%d#%s-%d", auth, s.Host, server.SsLocalPort, user.Name, n)
			} else {
				link = fmt.Sprintf("ss://%s@%s:%d#%s-%d", auth, server.Host, server.SsRemotePort, user.Name, n)
			}
			r.ShadowsocksLinks = append(r.ShadowsocksLinks, link)
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		user.ShadowsocksPassword = d.GenerateUserPassword()
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
