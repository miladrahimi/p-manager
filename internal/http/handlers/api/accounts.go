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
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
)

type AccountsStoreRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=32"`
	Enabled bool    `json:"enabled"`
	Quota   float64 `json:"quota" validate:"min=0"`
	Usage   float64 `json:"usage" validate:"min=0"`
}

// AccountShow returns the account info of an account.
func AccountShow(composer *composer.Composer, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		accountId := c.Param("accountId")
		if accountId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Account is required.",
			})
		}

		account := db.Data().FindAccountById(accountId)
		if account == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Account not found.",
			})
		}

		d := db.Data()
		trafficRatio := d.MainSettings.TrafficRatio

		r := AccountResponse{Account: *account, Proxies: make(map[string]string), Host: d.MainSettings.Host}
		r.Account.Usage = r.Account.Usage * trafficRatio
		r.Account.Quota = r.Account.Quota * trafficRatio

		r.Proxies = composer.AccountLinks(account)

		return c.JSON(http.StatusOK, r)
	}
}

type AccountsUpdateRequest struct {
	AccountsStoreRequest
}

type AccountsUpdatePartialRequest struct {
	Usage   *float64 `json:"usage"`
	Enabled *bool    `json:"enabled"`
}

type AccountsImportRequest struct {
	Url      string `json:"url" validate:"required,url"`
	Password string `json:"password" validate:"required"`
}

type AccountResponse struct {
	Account data.Account      `json:"account"`
	Proxies map[string]string `json:"proxies"`
	Host    string            `json:"host"`
}

// AccountsIndex returns the list of accounts.
func AccountsIndex(db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, db.Data().Accounts)
	}
}

// AccountsStore stores a new account.
func AccountsStore(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request AccountsStoreRequest
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

		if len(db.Data().Accounts) >= config.MaxAccountsCount {
			return c.JSON(http.StatusForbidden, map[string]string{
				"message": "You have already reached the maximum number of accounts.",
			})
		}

		for _, u := range db.Data().Accounts {
			if u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		proxyId := util.Uuid()
		account := data.NewAccount(
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

		db.Data().Accounts = append(db.Data().Accounts, account)

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, account)
	}
}

// AccountsUpdate updates an account.
func AccountsUpdate(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request AccountsUpdateRequest
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

		var account *data.Account
		for i, u := range db.Data().Accounts {
			if u.Id == c.Param("id") {
				account = db.Data().Accounts[i]
			}
		}
		if account == nil {
			return c.NoContent(http.StatusNotFound)
		}

		for _, u := range db.Data().Accounts {
			if u.Id != account.Id && u.Name == request.Name {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"message": "The name is already taken.",
				})
			}
		}

		account.Name = request.Name
		account.Quota = request.Quota
		account.Enabled = request.Enabled
		account.Usage = request.Usage
		account.UsageBytes = util.GB2Bytes(request.Usage)

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}

// AccountsUpdatePartial updates an account partially.
func AccountsUpdatePartial(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request AccountsUpdatePartialRequest
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

		var account *data.Account
		for i, u := range db.Data().Accounts {
			if u.Id == c.Param("id") {
				account = db.Data().Accounts[i]
			}
		}
		if account == nil {
			return c.NoContent(http.StatusNotFound)
		}

		if request.Usage != nil {
			account.Usage = *request.Usage
			account.UsageBytes = util.GB2Bytes(*request.Usage)
		}
		if request.Enabled != nil {
			account.Enabled = *request.Enabled
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}

// AccountsUpdatePartialBatch updates multiple accounts partially.
func AccountsUpdatePartialBatch(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request AccountsUpdatePartialRequest
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

		for _, account := range db.Data().Accounts {
			if request.Usage != nil {
				account.Usage = *request.Usage
				account.UsageBytes = util.GB2Bytes(*request.Usage)
			}
			if request.Enabled != nil {
				account.Enabled = *request.Enabled
			}
		}

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// AccountsDelete deletes an account.
func AccountsDelete(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		for i, u := range db.Data().Accounts {
			if u.Id == c.Param("id") {
				db.Data().Accounts = slices.Delete(db.Data().Accounts, i, i+1)
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

// AccountsDeleteBatch deletes multiple accounts.
func AccountsDeleteBatch(coordinator *coordinator.Coordinator, db *database.Database[data.Data]) echo.HandlerFunc {
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

		var newAccounts []*data.Account

		if enabled != nil {
			for _, u := range db.Data().Accounts {
				if u.Enabled != *enabled {
					newAccounts = append(newAccounts, u)
				}
			}
			db.Data().Accounts = newAccounts
		}

		if newAccounts == nil {
			newAccounts = []*data.Account{}
		}

		db.Data().Accounts = newAccounts

		if err := db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// AccountsImport imports accounts from another P-Manager.
func AccountsImport(
	coordinator *coordinator.Coordinator,
	db *database.Database[data.Data],
	hc *client.Client,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r AccountsImportRequest
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
		url := fmt.Sprintf("%s/api/accounts", baseURL)
		response, err := hc.Do("GET", url, r.Password, nil)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Request failed, err: %v", err.Error()),
			})
		}

		var accounts []data.Account
		if err = json.Unmarshal(response, &accounts); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message":  fmt.Sprintf("Invalid Response, err: %v", err.Error()),
				"response": string(response),
			})
		}

		var ids []string
		for _, u := range db.Data().Accounts {
			ids = append(ids, u.Id)
		}

		var proxyIds []string
		for _, u := range db.Data().Accounts {
			if u.ProxyId != "" {
				proxyIds = append(proxyIds, u.ProxyId)
			}
		}

		var names []string
		for _, u := range db.Data().Accounts {
			names = append(names, u.Name)
		}

		var results []string
		for _, u := range accounts {
			if slices.Index(ids, u.Id) != -1 {
				results = append(results, fmt.Sprintf("Skipped: DuplicateId=%s", u.Id))
				continue
			}
			if slices.Index(proxyIds, u.ProxyId) != -1 {
				results = append(results, fmt.Sprintf("Skipped: ID=%s DuplicateProxyId=%s", u.Id, u.ProxyId))
				continue
			}
			if slices.Index(names, u.Name) != -1 {
				results = append(results, fmt.Sprintf("Skipped: ID=%s DuplicateName=%s", u.Id, u.Name))
				continue
			}
			db.Data().Accounts = append(db.Data().Accounts, &u)
			results = append(results, fmt.Sprintf("Imported: ID=%s Name=%s", u.Id, u.Name))
		}

		if err = db.Save(); err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, results)
	}
}
