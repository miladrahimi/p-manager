package v1

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

type UsersStoreRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=32"`
	Enabled bool    `json:"enabled"`
	Quota   float64 `json:"quota" validate:"min=0"`
	Usage   float64 `json:"usage" validate:"min=0"`
}

type UsersUpdateRequest struct {
	UsersStoreRequest
}

type UsersUpdatePartialRequest struct {
	Usage   *float64 `json:"usage"`
	Enabled *bool    `json:"enabled"`
}

func UsersIndex(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data().Users)
	}
}

func UsersStore(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request UsersStoreRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		if len(d.Data().Users) >= config.MaxUsersCount {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You have already reached the maximum number of users.",
			})
		}

		for _, u := range d.Data().Users {
			if u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		user := data.NewUser(
			d.Data().GenerateUserId(),
			d.Data().GenerateUserUuid(),
			request.Name,
			request.Quota,
			request.Usage,
			util.GB2Bytes(request.Usage),
			time.Now().Unix(),
			request.Enabled,
			d.Data().GenerateUserPassword(),
			config.ShadowsocksMethod,
			time.Now().UnixMilli(),
		)

		d.Data().Users = append(d.Data().Users, user)

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, user)
	}
}

func UsersUpdate(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request UsersUpdateRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		var user *data.User
		for i, u := range d.Data().Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				user = d.Data().Users[i]
			}
		}
		if user == nil {
			return c.NoContent(http.StatusNotFound)
		}

		for _, u := range d.Data().Users {
			if u.Id != user.Id && u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		user.Name = request.Name
		user.Quota = request.Quota
		user.Enabled = request.Enabled
		user.Usage = request.Usage
		user.UsageBytes = util.GB2Bytes(request.Usage)

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

func UsersUpdatePartial(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request UsersUpdatePartialRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		var user *data.User
		for i, u := range d.Data().Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				user = d.Data().Users[i]
			}
		}
		if user == nil {
			return c.NoContent(http.StatusNotFound)
		}

		if request.Usage != nil {
			user.Usage = *request.Usage
			user.UsageBytes = util.GB2Bytes(*request.Usage)
		}
		if request.Enabled != nil {
			user.Enabled = *request.Enabled
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

func UsersUpdatePartialBatch(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request UsersUpdatePartialRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		for _, user := range d.Data().Users {
			if request.Usage != nil {
				user.Usage = *request.Usage
				user.UsageBytes = util.GB2Bytes(*request.Usage)
			}
			if request.Enabled != nil {
				user.Enabled = *request.Enabled
			}
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func UsersDelete(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		for i, u := range d.Data().Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				d.Data().Users = slices.Delete(d.Data().Users, i, i+1)
				if err := d.Save(); err != nil {
					return errors.WithStack(err)
				}
				go coordinator.UpdateConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func UsersDeleteBatch(coordinator *coordinator.Coordinator, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		enabledParam := c.QueryParam("enabled")
		if enabledParam != "" && enabledParam != "true" && enabledParam != "false" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Invalid query parameter.",
			})
		}

		var enabled *bool
		if enabledParam != "" {
			enabledBool := enabledParam == "true"
			enabled = &enabledBool
		}

		var newUsers []*data.User

		if enabled != nil {
			for _, u := range d.Data().Users {
				if u.Enabled != *enabled {
					newUsers = append(newUsers, u)
				}
			}
			d.Data().Users = newUsers
		}

		if newUsers == nil {
			newUsers = []*data.User{}
		}

		d.Data().Users = newUsers

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}
