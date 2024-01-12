package handlers

import (
	b64 "encoding/base64"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/url"
	"shadowsocks-manager/internal/database"
	"strings"
)

func SSConf(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		jIndex := strings.Index(c.Request().RequestURI, ".json")
		p, _ := url.QueryUnescape(c.Request().RequestURI[8:jIndex])
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

		return c.JSON(http.StatusOK, map[string]interface{}{
			"server":      d.Data.Settings.ShadowsocksHost,
			"server_port": d.Data.Settings.ShadowsocksPort,
			"password":    parts[1],
			"method":      parts[0],
		})
	}
}
