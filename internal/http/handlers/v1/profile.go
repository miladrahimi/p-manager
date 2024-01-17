package v1

import (
	"encoding/base64"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/random"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"net/http"
)

type ProfileResponse struct {
	User            database.User `json:"user"`
	Used            float64       `json:"used"`
	ShadowsocksLink string        `json:"shadowsocks_link"`
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

		s := d.Data.Settings
		auth := base64.StdEncoding.EncodeToString([]byte(user.ShadowsocksMethod + ":" + user.ShadowsocksPassword))
		link := fmt.Sprintf("ss://%s@%s:%d#%s", auth, s.Host, s.ShadowsocksPort, user.Name)
		used := utils.RoundFloat(user.Used*d.Data.Settings.TrafficRatio, 2)

		r := ProfileResponse{User: *user, ShadowsocksLink: link, Used: used}
		r.User.Used = 0
		r.User.Quota = int(float64(r.User.Quota) * d.Data.Settings.TrafficRatio)

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

		var newPassword string
		for {
			newPassword = random.String(16)
			isUnique := true
			for _, u := range d.Data.Users {
				if u.ShadowsocksPassword == newPassword {
					isUnique = false
					break
				}
			}
			if isUnique {
				break
			}
		}
		user.ShadowsocksPassword = newPassword
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}
