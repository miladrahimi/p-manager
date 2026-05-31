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
	"github.com/miladrahimi/p-node/pkg/http/client"
)

type AccountsStoreRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=32"`
	Enabled bool    `json:"enabled"`
	Quota   float64 `json:"quota" validate:"min=0"`
	Usage   float64 `json:"usage" validate:"min=0"`
}

// AccountShow returns the account info of an account.
func AccountShow(composer *composer.Composer, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		accountId := c.Param("accountId")
		if accountId == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Account is required.",
			})
		}

		var account *data.Account
		var r AccountResponse
		db.Read(func(d *data.Data) {
			account = d.FindAccountById(accountId)
			if account == nil {
				return
			}
			r = AccountResponse{Account: *account, Proxies: make(map[string]string), Host: d.MainSettings.Host}
			r.Account.Usage = r.Account.Usage * d.MainSettings.TrafficRatio
			r.Account.Quota = r.Account.Quota * d.MainSettings.TrafficRatio
		})
		if account == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"message": "Account not found.",
			})
		}

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
func AccountsIndex(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var accounts []data.Account
		db.Read(func(d *data.Data) {
			accounts = make([]data.Account, len(d.Accounts))
			for i, a := range d.Accounts {
				accounts[i] = *a
			}
		})
		return c.JSON(http.StatusOK, accounts)
	}
}

// AccountsStore stores a new account.
func AccountsStore(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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

		account := data.NewAccount(
			util.Uuid(),
			util.Uuid(),
			request.Name,
			request.Quota,
			request.Usage,
			util.GB2Bytes(request.Usage),
			time.Now().Unix(),
			request.Enabled,
			time.Now().UnixMilli(),
		)

		var failure *httpResult
		err := db.Mutate(func(d *data.Data) (bool, error) {
			if len(d.Accounts) >= config.MaxAccountsCount {
				failure = &httpResult{http.StatusForbidden, "You have already reached the maximum number of accounts."}
				return false, nil
			}
			for _, u := range d.Accounts {
				if u.Name == request.Name {
					failure = &httpResult{http.StatusBadRequest, "The name is already taken."}
					return false, nil
				}
			}
			d.Accounts = append(d.Accounts, account)
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if failure != nil {
			return failure.write(c)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusCreated, account)
	}
}

// AccountsUpdate updates an account.
func AccountsUpdate(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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

		var account data.Account
		found := false
		var failure *httpResult
		err := db.Mutate(func(d *data.Data) (bool, error) {
			var target *data.Account
			for i, u := range d.Accounts {
				if u.Id == c.Param("id") {
					target = d.Accounts[i]
				}
			}
			if target == nil {
				return false, nil
			}

			for _, u := range d.Accounts {
				if u.Id != target.Id && u.Name == request.Name {
					failure = &httpResult{http.StatusBadRequest, "The name is already taken."}
					return false, nil
				}
			}

			target.Name = request.Name
			target.Quota = request.Quota
			target.Enabled = request.Enabled
			target.Usage = request.Usage
			target.UsageBytes = util.GB2Bytes(request.Usage)
			found = true
			account = *target
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if failure != nil {
			return failure.write(c)
		}
		if !found {
			return c.NoContent(http.StatusNotFound)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}

// AccountsUpdatePartial updates an account partially.
func AccountsUpdatePartial(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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

		var account data.Account
		found := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			var target *data.Account
			for i, u := range d.Accounts {
				if u.Id == c.Param("id") {
					target = d.Accounts[i]
				}
			}
			if target == nil {
				return false, nil
			}

			if request.Usage != nil {
				target.Usage = *request.Usage
				target.UsageBytes = util.GB2Bytes(*request.Usage)
			}
			if request.Enabled != nil {
				target.Enabled = *request.Enabled
			}
			found = true
			account = *target
			return true, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if !found {
			return c.NoContent(http.StatusNotFound)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, account)
	}
}

// AccountsUpdatePartialBatch updates multiple accounts partially.
func AccountsUpdatePartialBatch(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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

		err := db.Write(func(d *data.Data) {
			for _, account := range d.Accounts {
				if request.Usage != nil {
					account.Usage = *request.Usage
					account.UsageBytes = util.GB2Bytes(*request.Usage)
				}
				if request.Enabled != nil {
					account.Enabled = *request.Enabled
				}
			}
		})
		if err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// AccountsDelete deletes an account.
func AccountsDelete(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		deleted := false
		err := db.Mutate(func(d *data.Data) (bool, error) {
			for i, u := range d.Accounts {
				if u.Id == c.Param("id") {
					d.Accounts = slices.Delete(d.Accounts, i, i+1)
					deleted = true
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if deleted {
			go coordinator.UpdateConfigs()
		}

		return c.NoContent(http.StatusNoContent)
	}
}

// AccountsDeleteBatch deletes multiple accounts.
func AccountsDeleteBatch(coordinator *coordinator.Coordinator, db *data.Store) echo.HandlerFunc {
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

		err := db.Write(func(d *data.Data) {
			newAccounts := []*data.Account{}
			if enabled != nil {
				for _, u := range d.Accounts {
					if u.Enabled != *enabled {
						newAccounts = append(newAccounts, u)
					}
				}
			}
			d.Accounts = newAccounts
		})
		if err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

// AccountsImport imports accounts from another P-Manager.
func AccountsImport(
	coordinator *coordinator.Coordinator,
	db *data.Store,
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

		var results []string
		err = db.Write(func(d *data.Data) {
			ids := make([]string, 0, len(d.Accounts))
			proxyIds := make([]string, 0, len(d.Accounts))
			names := make([]string, 0, len(d.Accounts))
			for _, u := range d.Accounts {
				ids = append(ids, u.Id)
				if u.ProxyId != "" {
					proxyIds = append(proxyIds, u.ProxyId)
				}
				names = append(names, u.Name)
			}

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
				imported := u
				d.Accounts = append(d.Accounts, &imported)
				results = append(results, fmt.Sprintf("Imported: ID=%s Name=%s", u.Id, u.Name))
			}
		})
		if err != nil {
			return errors.WithStack(err)
		}

		go coordinator.UpdateConfigs()

		return c.JSON(http.StatusOK, results)
	}
}
