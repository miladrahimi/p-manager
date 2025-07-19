package v1

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/licensor"
	"github.com/miladrahimi/p-manager/internal/utils"
	"net/http"
	"slices"
	"strconv"
	"time"
)

type UsersStoreRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=32"`
	Enabled bool    `json:"enabled"`
	Quota   float64 `json:"quota" validate:"min=0"`
	Usage   float64 `json:"usage"`
}

type UsersUpdateRequest struct {
	UsersStoreRequest
}

type UsersUpdatePartialRequest struct {
	Usage   *float64 `json:"usage"`
	Enabled *bool    `json:"enabled"`
}

func UsersIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Content.Users)
	}
}

func UsersStore(coordinator *coordinator.Coordinator, d *database.Database, l *licensor.Licensor) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		if len(d.Content.Users) >= config.MaxUsersCount {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You have already reached the maximum number of users.",
			})
		}
		if len(d.Content.Users) >= config.FreeUsersCount && !l.Licensed() {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You cannot add more users without license.",
			})
		}

		for _, u := range d.Content.Users {
			if u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		user := &database.User{}
		user.Id = d.GenerateUserId()
		user.Identity = d.GenerateUserIdentity()
		user.CreatedAt = time.Now().UnixMilli()
		user.ShadowsocksMethod = config.ShadowsocksMethod
		user.ShadowsocksPassword = d.GenerateUserPassword()
		user.Usage = request.Usage
		user.UsageBytes = utils.GB2Bytes(request.Usage)
		user.Name = request.Name
		user.Quota = request.Quota
		user.Enabled = request.Enabled

		d.Content.Users = append(d.Content.Users, user)

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusCreated, user)
	}
}

func UsersUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		var user *database.User
		for i, u := range d.Content.Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				user = d.Content.Users[i]
			} else {
				if u.Name == request.Name {
					return c.JSON(http.StatusBadRequest, map[string]string{
						"message": "The name is already taken.",
					})
				}
			}
		}
		if user == nil {
			return c.NoContent(http.StatusNotFound)
		}

		user.Name = request.Name
		user.Quota = request.Quota
		user.Enabled = request.Enabled

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

func UsersUpdatePartial(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		var user *database.User
		for i, u := range d.Content.Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				user = d.Content.Users[i]
			}
		}
		if user == nil {
			return c.NoContent(http.StatusNotFound)
		}

		if request.Usage != nil {
			user.Usage = *request.Usage
			user.UsageBytes = utils.GB2Bytes(*request.Usage)
		}
		if request.Enabled != nil {
			user.Enabled = true
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

func UsersUpdatePartialBatch(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		for _, user := range d.Content.Users {
			if request.Usage != nil {
				user.Usage = *request.Usage
				user.UsageBytes = utils.GB2Bytes(*request.Usage)
			}
			if request.Enabled != nil {
				user.Enabled = *request.Enabled
			}
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func UsersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		for i, u := range d.Content.Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				d.Content.Users = slices.Delete(d.Content.Users, i, i+1)
				if err := d.Save(); err != nil {
					return errors.WithStack(err)
				}
				go coordinator.SyncConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func UsersDeleteBatch(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		var newUsers []*database.User

		if enabled != nil {
			for _, u := range d.Content.Users {
				if u.Enabled != *enabled {
					newUsers = append(newUsers, u)
				}
			}
			d.Content.Users = newUsers
		}

		if newUsers == nil {
			newUsers = []*database.User{}
		}

		d.Content.Users = newUsers

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}
