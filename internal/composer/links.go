package composer

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
)

// UserLinks builds all available proxy links for a user.
func (w *Composer) UserLinks(user *data.User) map[string]string {
	d := w.db.Data()
	xs := d.XraySettings
	links := make(map[string]string)

	addLink := func(name, host string, port int, sni string) {
		if host == "" || port <= 0 {
			return
		}
		links[name] = buildVrrvLink(host, port, user.VlessId, xs.VrrvPublicKey, sni, name)
	}

	addLink("vrrv_direct", d.MainSettings.Host, xs.VrrvDirectPort, xs.ManagerSni)
	addLink("vrrv_2_vrrv_relay", d.MainSettings.Host, xs.Vrrv2VrrvPort, xs.ManagerSni)

	if xs.VrrvRemotePort > 0 {
		for _, n := range d.Nodes {
			name := fmt.Sprintf("vrrv_remote_%s", strings.ReplaceAll(n.Host, ".", "_"))
			addLink(name, n.Host, xs.VrrvRemotePort, xs.NodeSni)
		}
	}

	return links
}

func buildVrrvLink(host string, port int, userId string, publicKey string, nodeSni string, tag string) string {
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
	query.Set("sni", nodeSni)
	query.Set("pbk", publicKey)

	vlessUrl.RawQuery = query.Encode()

	return vlessUrl.String()
}
