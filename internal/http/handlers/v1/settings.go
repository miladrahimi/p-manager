package v1

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"io"
	"net/http"
	"time"
)

func SettingsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, d.Data.Settings)
	}
}

func SettingsUpdate(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var settings database.Settings
		if err := c.Bind(&settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Cannot parse the request body.",
			})
		}
		if err := validator.New().Struct(settings); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": fmt.Sprintf("Validation error: %v", err.Error()),
			})
		}

		d.Data.Settings = &settings
		d.Save()

		go coordinator.SyncConfigs()
		go coordinator.Report()

		return c.JSON(http.StatusOK, settings)
	}
}

func SettingsStatsShow(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			*database.Stats
			UsersCount int `json:"users_count"`
		}{
			Stats:      d.Data.Stats,
			UsersCount: len(d.Data.Users),
		})
	}
}

func SettingsStatsZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Stats.Traffic = 0
		d.Data.Stats.UpdatedAt = time.Now().UnixMilli()
		d.Save()
		return c.JSON(http.StatusOK, d.Data.Stats)
	}
}

func SettingsServersZero(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, s := range d.Data.Servers {
			s.Traffic = 0
		}
		d.Save()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsUsersZero(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		for _, u := range d.Data.Users {
			u.Used = 0
			u.UsedBytes = 0
			u.Enabled = true
		}
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

func SettingsUsersDelete(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Data.Users = []*database.User{}
		d.Save()

		go coordinator.SyncConfigs()

		return c.NoContent(http.StatusNoContent)
	}
}

type Import struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type ImportedKey struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Cipher    string `json:"cipher"`
	Secret    string `json:"secret"`
	Name      string `json:"name"`
	Quota     int    `json:"quota"`
	CreatedAt int64  `json:"created_at"`
	Enabled   bool   `json:"enabled"`
	Used      int    `json:"used"`
}

func SettingsImport(coordinator *coordinator.Coordinator, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r Import
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

		client := &http.Client{}
		req, err := http.NewRequest("GET", r.URL+"/v1/keys", nil)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("http.NewRequest: %s", err.Error()),
			})
		}
		req.Header.Set("Authorization", "Bearer "+r.Token)

		resp, err := client.Do(req)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("client.Do: %s", err.Error()),
			})
		}
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("io.ReadAll: %s", err.Error()),
			})
		}

		var Keys []ImportedKey
		err = json.Unmarshal(body, &Keys)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("json.Unmarshal: %s", err.Error()),
			})
		}

		for _, key := range Keys {
			exist := false
			for _, u := range d.Data.Users {
				if u.ShadowsocksPassword == key.Secret {
					exist = true
				}
				if u.Identity == key.Code {
					exist = true
				}
			}
			if exist {
				continue
			}

			user := &database.User{}
			user.Id = d.GenerateUserId()
			user.Identity = key.Code
			user.ShadowsocksMethod = key.Cipher
			user.ShadowsocksPassword = key.Secret
			user.CreatedAt = key.CreatedAt
			user.Used = utils.RoundFloat(float64(key.Used)/1000, 2)
			user.UsedBytes = int64(user.Used * 1000 * 1000 * 1000)
			user.Name = key.Name
			user.Quota = key.Quota / 1000
			user.Enabled = key.Enabled
			d.Data.Users = append(d.Data.Users, user)
		}

		d.Save()

		go coordinator.SyncConfigs()

		return c.JSON(http.StatusOK, map[string]string{
			"message": "success",
		})
	}
}
