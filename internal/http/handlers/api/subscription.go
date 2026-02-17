package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

type SubscriptionItem struct {
	Remarks   string                `json:"remarks"`
	Outbounds []*component.Outbound `json:"outbounds"`
}

// SubscriptionShow returns the proxy subscription for a user proxy ID.
func SubscriptionShow(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		proxyId := c.Param("proxyId")
		if proxyId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Proxy ID is required.",
			})
		}

		u := db.Data().FindUserByProxyId(proxyId)
		if u == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "User not found.",
			})
		}

		d := db.Data()
		xs := d.XraySettings

		var items []SubscriptionItem
		addRrItem := func(name, host string, port int, sni string) {
			if host == "" || port <= 0 {
				return
			}
			nameWithHost := fmt.Sprintf("%s@%s", name, d.MainSettings.Host)
			items = append(items, SubscriptionItem{
				Remarks: nameWithHost,
				Outbounds: []*component.Outbound{
					vless.MakeRrOutbound(
						nameWithHost,
						host,
						port,
						u.ProxyId,
						xs.RealityPublicKey,
						sni,
					),
				},
			})
		}

		addRrItem("direct-rr", d.MainSettings.Host, xs.DirectRrPort, xs.ManagerSni)
		addRrItem("relay-rr2rr", d.MainSettings.Host, xs.RelayRr2RrPort, xs.ManagerSni)
		addRrItem("relay-rr2ssh", d.MainSettings.Host, xs.RelayRr2SshPort, xs.ManagerSni)

		if xs.RemoteRrPort > 0 {
			for _, n := range d.Nodes {
				name := fmt.Sprintf("remote-rr-%s", n.Host)
				addRrItem(name, n.Host, xs.RemoteRrPort, xs.NodeSni)
			}
		}

		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")

		return c.JSON(http.StatusOK, items)
	}
}
