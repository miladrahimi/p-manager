package v1

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"net/http"
	"slices"
)

type SettingsImportPManagerRequest struct {
	Url      string `json:"url" validate:"required,url"`
	Password string `json:"password" validate:"required"`
}

func ImportsStore(d *database.Database, hc *client.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r SettingsImportPManagerRequest
		if err := c.Bind(&r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(r); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		url := fmt.Sprintf("%s/v1/users", r.Url)
		response, err := hc.Do("GET", url, r.Password, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Request failed, err: %v", err.Error()),
			})
		}

		var users []database.User
		if err = json.Unmarshal(response, &users); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message":  fmt.Sprintf("Invalid Response, err: %v", err.Error()),
				"response": string(response),
			})
		}

		d.Locker.Lock()
		defer d.Locker.Unlock()

		var names []string
		for _, u := range d.Content.Users {
			names = append(names, u.Name)
		}

		var results []string
		for i, u := range users {
			if slices.Index(names, u.Name) != -1 {
				results = append(results, fmt.Sprintf("Ignored #%d: DuplicateName=%s", u.Id, u.Name))
				continue
			}
			u.Id = d.GenerateUserId()
			d.Content.Users = append(d.Content.Users, &u)
			results = append(results, fmt.Sprintf("Imported #%d: ID=%d Name=%s", users[i].Id, u.Id, u.Name))
		}

		if err = d.Save(); err != nil {
			return errors.WithStack(err)
		}

		return c.JSON(http.StatusOK, results)
	}
}
