package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
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

type UsersImportRequest struct {
	Url      string `json:"url" validate:"required,url"`
	Password string `json:"password" validate:"required"`
}

// UsersIndex returns the list of users.
func UsersIndex(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, db.Data().Users)
	}
}

// UsersStore stores a new user.
func UsersStore(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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

		if len(db.Data().Users) >= config.MaxUsersCount {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You have already reached the maximum number of users.",
			})
		}

		for _, u := range db.Data().Users {
			if u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		proxyId := util.Uuid()
		user := data.NewUser(
			util.Uuid(),
			proxyId,
			request.Name,
			request.Quota,
			request.Usage,
			util.GB2Bytes(request.Usage),
			time.Now().Unix(),
			request.Enabled,
			time.Now().UnixMilli(),
		)

		db.Data().Users = append(db.Data().Users, user)

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, user)
	}
}

// UsersUpdate updates a user.
func UsersUpdate(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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
		for i, u := range db.Data().Users {
			if u.Id == c.Param("id") {
				user = db.Data().Users[i]
			}
		}
		if user == nil {
			return c.NoContent(http.StatusNotFound)
		}

		for _, u := range db.Data().Users {
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

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

// UsersUpdatePartial updates a user partially.
func UsersUpdatePartial(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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
		for i, u := range db.Data().Users {
			if u.Id == c.Param("id") {
				user = db.Data().Users[i]
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

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, user)
	}
}

// UsersUpdatePartialBatch updates multiple users partially.
func UsersUpdatePartialBatch(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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

		for _, user := range db.Data().Users {
			if request.Usage != nil {
				user.Usage = *request.Usage
				user.UsageBytes = util.GB2Bytes(*request.Usage)
			}
			if request.Enabled != nil {
				user.Enabled = *request.Enabled
			}
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// UsersDelete deletes a user.
func UsersDelete(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		for i, u := range db.Data().Users {
			if u.Id == c.Param("id") {
				db.Data().Users = slices.Delete(db.Data().Users, i, i+1)
				if err := db.Save(); err != nil {
					return errors.WithStack(err)
				}
				go coordinator.UpdateConfigs()
				break
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}

// UsersDeleteBatch deletes multiple users.
func UsersDeleteBatch(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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
			for _, u := range db.Data().Users {
				if u.Enabled != *enabled {
					newUsers = append(newUsers, u)
				}
			}
			db.Data().Users = newUsers
		}

		if newUsers == nil {
			newUsers = []*data.User{}
		}

		db.Data().Users = newUsers

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// UsersImport imports users from another P-Manager.
func UsersImport(
	coordinator *coordinator.Coordinator,
	db *database.Database[data.Data],
	hc *client.Client,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r UsersImportRequest
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

		baseURL := strings.TrimRight(r.Url, "/")
		url := fmt.Sprintf("%s/api/users", baseURL)
		response, err := hc.Do("GET", url, r.Password, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Request failed, err: %v", err.Error()),
			})
		}

		var users []data.User
		if err = json.Unmarshal(response, &users); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message":  fmt.Sprintf("Invalid Response, err: %v", err.Error()),
				"response": string(response),
			})
		}

		var ids []string
		for _, u := range db.Data().Users {
			ids = append(ids, u.Id)
		}

		var proxyIds []string
		for _, u := range db.Data().Users {
			proxyId := u.ProxyId
			if proxyId == "" {
				proxyId = u.VlessId
			}
			if proxyId != "" {
				proxyIds = append(proxyIds, proxyId)
			}
		}

		var names []string
		for _, u := range db.Data().Users {
			names = append(names, u.Name)
		}

		var results []string
		for _, u := range users {
			if slices.Index(ids, u.Id) != -1 {
				results = append(results, fmt.Sprintf("Skipped: DuplicateId=%s", u.Id))
				continue
			}
			if u.ProxyId == "" {
				u.ProxyId = u.VlessId
			}
			if u.VlessId == "" {
				u.VlessId = u.ProxyId
			}
			if slices.Index(proxyIds, u.ProxyId) != -1 {
				results = append(results, fmt.Sprintf("Skipped: ID=%s DuplicateProxyId=%s", u.Id, u.ProxyId))
				continue
			}
			if slices.Index(names, u.Name) != -1 {
				results = append(results, fmt.Sprintf("Skipped: ID=%s DuplicateName=%s", u.Id, u.Name))
				continue
			}
			db.Data().Users = append(db.Data().Users, &u)
			results = append(results, fmt.Sprintf("Imported: ID=%s Name=%s", u.Id, u.Name))
		}

		if err = db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, results)
	}
}
