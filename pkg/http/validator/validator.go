package validator

import (
	pg "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Validator struct {
	validator *pg.Validate
}

func (cv *Validator) Validate(i interface{}) error {
	v := pg.New(pg.WithRequiredStructEnabled())
	if err := v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func New() *Validator {
	return &Validator{validator: pg.New()}
}
