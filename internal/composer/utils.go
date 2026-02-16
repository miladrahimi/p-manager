package composer

import (
	"net"
	"net/url"
	"strconv"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
)

// BuildRrLink builds a vless link for clients.
func buildRrLink(host string, port int, userId string, publicKey string, nodeSni string, tag string) string {
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
