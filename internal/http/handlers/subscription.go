package handlers

import (
	b64 "encoding/base64"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/url"
	"shadowsocks-manager/internal/database"
	"strings"
)

func Subscription(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		p, _ := url.QueryUnescape(c.Request().RequestURI[14:])
		var auth, err = b64.StdEncoding.DecodeString(p)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		parts := strings.Split(string(auth), ":")
		if len(parts) != 2 {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Not found.",
			})
		}

		var lines []string
		lines = append(lines, fmt.Sprintf(
			"ss://%s@%s:%d/?outline=1",
			b64.StdEncoding.EncodeToString([]byte(parts[0]+":"+parts[1])),
			d.Data.Settings.ShadowsocksHost,
			d.Data.Settings.ShadowsocksPort,
		))

		return c.String(http.StatusOK, b64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n"))))
	}
}
