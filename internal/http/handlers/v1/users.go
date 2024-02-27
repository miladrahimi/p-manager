package v1

import (
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"net/http"
	"slices"
	"strconv"
	"time"
)

type UsersStoreRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=32"`
	Enabled bool    `json:"enabled"`
	Quota   float64 `json:"quota" validate:"min=0"`
	Used    float64 `json:"used"`
}

type UsersUpdateRequest struct {
	UsersStoreRequest
	Id int `json:"id"`
}

func UsersIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Users)
	}
}

func UsersStore(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
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

		if len(d.Data.Users) >= config.LimitedUsersCount && !coordinator.Licensed() {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You cannot add more users without premium license.",
			})
		}

		for _, u := range d.Data.Users {
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
		user.Used = request.Used
		user.UsedBytes = int64(request.Used * 1000 * 1000 * 1000)
		user.Name = request.Name
		user.Quota = request.Quota
		user.Enabled = request.Enabled

		d.Data.Users = append(d.Data.Users, user)
		d.Save()

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

		var user *database.User
		for i, u := range d.Data.Users {
			if u.Id == request.Id {
				user = d.Data.Users[i]
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
		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

func KeysZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, u := range d.Data.Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				u.Used = 0
				d.Save()
				return c.NoContent(http.StatusNoContent)
			}
		}
		return c.NoContent(http.StatusNotFound)
	}
}

func UsersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for i, u := range d.Data.Users {
			if strconv.Itoa(u.Id) == c.Param("id") {
				d.Data.Users = slices.Delete(d.Data.Users, i, i+1)
				d.Save()
				go coordinator.SyncConfigs()
			}
		}
		return c.NoContent(http.StatusNoContent)
	}
}
