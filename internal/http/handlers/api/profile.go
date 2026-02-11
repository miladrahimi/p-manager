package api

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

type ProfileResponse struct {
	User    data.User         `json:"user"`
	Proxies map[string]string `json:"proxies"`
}

func ProfileShow(db *database.Database[data.Data]) echo.HandlerFunc {
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
		VrrvPublicKey := d.XraySettings.VrrvPublicKey
		TrafficRatio := d.MainSettings.TrafficRatio

		r := ProfileResponse{User: *u, Proxies: make(map[string]string)}
		r.User.Usage = r.User.Usage * TrafficRatio
		r.User.Quota = r.User.Quota * TrafficRatio

		if d.XraySettings.VrrvDirectPort > 0 {
			name := "vrrv_direct"
			port := d.XraySettings.VrrvDirectPort
			r.Proxies[name] = buildVrrvLink(d.MainSettings.Host, port, r.User.VlessId, VrrvPublicKey, name)
		}

		if d.XraySettings.Vrrv2VrrvPort > 0 {
			name := "vrrv_2_vrrv_relay"
			port := d.XraySettings.Vrrv2VrrvPort
			r.Proxies[name] = buildVrrvLink(d.MainSettings.Host, port, r.User.VlessId, VrrvPublicKey, name)
		}

		if d.XraySettings.VrrvRemotePort > 0 {
			port := d.XraySettings.VrrvRemotePort
			for _, n := range d.Nodes {
				name := fmt.Sprintf("vrrv_remote_%s", strings.Replace(n.Host, ".", "_", -1))
				r.Proxies[name] = buildVrrvLink(n.Host, port, u.VlessId, VrrvPublicKey, name)
			}
		}

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

// buildVrrvLink builds a VRRV link for a user.
func buildVrrvLink(host string, port int, userId string, publicKey string, tag string) string {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	vlessUrl := url.URL{
		Scheme:   vless.Protocol,
		User:     url.User(userId),
		Host:     address,
		Fragment: tag,
	}

	query := url.Values{}
	query.Set("flow", vless.FlowVision)
	query.Set("encryption", vless.EncryptionNone)
	query.Set("type", vless.NetworkRaw)
	query.Set("security", vless.SecurityReality)
	query.Set("sni", vless.ServerNameStackOverflow)
	query.Set("pbk", publicKey)

	vlessUrl.RawQuery = query.Encode()

	return vlessUrl.String()
}
