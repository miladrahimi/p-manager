package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
)

// MainSettingsShow returns the main settings.
func MainSettingsShow(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var settings data.Settings
		db.Read(func(d *data.Data) {
			settings = *d.MainSettings
		})
		return c.JSON(http.StatusOK, &settings)
	}
}

// MainSettingsUpdate updates the main settings.
func MainSettingsUpdate(db *data.Store) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var r data.Settings
		if err := ctx.Bind(&r); err != nil {
			return ctx.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(r); err != nil {
			return ctx.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if err := db.Write(func(d *data.Data) {
			d.MainSettings = &r
		}); err != nil {
			return errors.WithStack(err)
		}

		return ctx.JSON(http.StatusOK, r)
	}
}
