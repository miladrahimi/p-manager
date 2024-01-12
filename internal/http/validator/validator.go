package validator

import (
	playground "github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Validator struct {
	validator *playground.Validate
}

func (cv *Validator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func New() *Validator {
	return &Validator{validator: playground.New()}
}
