package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
)

type StatsUpdatePartialRequest struct {
	TotalUsage *float64 `json:"total_usage" validate:"omitempty,min=0"`
}

type StatsResponse struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage"`
	TotalAccounts     int     `json:"total_accounts"`
	ActiveAccounts    int     `json:"active_accounts"`
}

// StatsIndex returns the statistics of the platform.
func StatsIndex(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var response StatsResponse
		db.Read(func(d *data.Data) {
			response = newStatsResponse(d)
		})
		return c.JSON(http.StatusOK, &response)
	}
}

// StatsUpdatePartial updates the statistics of the platform.
func StatsUpdatePartial(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		var request StatsUpdatePartialRequest
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

		var response StatsResponse
		err := db.Write(func(d *data.Data) {
			if request.TotalUsage != nil {
				d.Stats.TotalUsage = *request.TotalUsage
				d.Stats.TotalUsageBytes = util.GB2Bytes(*request.TotalUsage)
				d.Stats.TotalUsageResetAt = time.Now().UnixMilli()
			}
			response = newStatsResponse(d)
		})
		if err != nil {
			return errors.WithStack(err)
		}

		return c.JSON(http.StatusOK, &response)
	}
}

// newStatsResponse builds a stats response. The caller must hold the store lock.
func newStatsResponse(d *data.Data) StatsResponse {
	return StatsResponse{
		TotalUsageResetAt: d.Stats.TotalUsageResetAt,
		TotalUsage:        d.Stats.TotalUsage,
		TotalAccounts:     len(d.Accounts),
		ActiveAccounts:    d.CountActiveAccounts(),
	}
}
