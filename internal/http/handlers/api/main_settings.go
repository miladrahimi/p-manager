package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

// MainSettingsShow returns the main settings.
func MainSettingsShow(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, db.Data().MainSettings)
	}
}

// MainSettingsUpdate updates the main settings.
func MainSettingsUpdate(db *database.Database[data.Data]) echo.HandlerFunc {
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

		db.Data().MainSettings = &r
		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		return ctx.JSON(http.StatusOK, r)
	}
}
