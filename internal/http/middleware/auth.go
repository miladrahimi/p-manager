package middleware

import (
	"github.com/labstack/echo/v4"
	"strings"
	"xray-manager/internal/database"
)

// Authorize checks the HTTP headers.
func Authorize(d *database.Database) func(echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(context echo.Context) error {
			if !authorizeToken(d.Data.Settings.AdminPassword, context) {
				return echo.ErrUnauthorized
			}
			return next(context)
		}
	}
}

// authorizeToken checks the extracted token from HTTP headers.
func authorizeToken(token string, context echo.Context) bool {
	authHeader := context.Request().Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[len("Bearer "):] == token
	}
	return false
}
