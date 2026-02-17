package composer

import (
	"net"
	"net/url"
	"strconv"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
)

type vlessLinkOptions struct {
	host        string
	port        int
	userId      string
	tag         string
	flow        string
	network     string
	security    string
	sni         string
	publicKey   string
	path        string
	fingerprint string
}

func makeVlessLink(opts vlessLinkOptions) string {
	address := net.JoinHostPort(opts.host, strconv.Itoa(opts.port))
	vlessUrl := url.URL{
		Scheme:   vless.Protocol,
		User:     url.User(opts.userId),
		Host:     address,
		Fragment: opts.tag,
	}

	query := url.Values{}
	if opts.flow != "" {
		query.Set("flow", opts.flow)
	}
	query.Set("encryption", vless.EncryptionNone)
	if opts.network != "" {
		query.Set("type", opts.network)
	}
	if opts.security != "" {
		query.Set("security", opts.security)
	}
	if opts.sni != "" {
		query.Set("sni", opts.sni)
	}
	if opts.publicKey != "" {
		query.Set("pbk", opts.publicKey)
	}
	if opts.path != "" {
		query.Set("path", opts.path)
	}
	if opts.fingerprint != "" {
		query.Set("fp", opts.fingerprint)
	}

	vlessUrl.RawQuery = query.Encode()

	return vlessUrl.String()
}
