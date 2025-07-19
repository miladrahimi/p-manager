package v1

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/utils"
	"net/http"
	"time"
)

type StatsUpdatePartialRequest struct {
	TotalUsage *float64 `json:"total_usage" validate:"omitempty,min=0,max=0"`
}

type StatsResponse struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage"`
	TotalUsers        int     `json:"total_users"`
	ActiveUsers       int     `json:"active_users"`
}

func makeStatsResponse(d *database.Database) *StatsResponse {
	return &StatsResponse{
		TotalUsageResetAt: d.Content.Stats.TotalUsageResetAt,
		TotalUsage:        d.Content.Stats.TotalUsage,
		TotalUsers:        len(d.Content.Users),
		ActiveUsers:       d.CountActiveUsers(),
	}
}

func StatsIndex(d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, makeStatsResponse(d))
	}
}

func StatsUpdatePartial(d *database.Database) echo.HandlerFunc {
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

		d.Locker.Lock()
		defer d.Locker.Unlock()

		if request.TotalUsage != nil {
			d.Content.Stats.TotalUsage = *request.TotalUsage
			d.Content.Stats.TotalUsageBytes = utils.GB2Bytes(*request.TotalUsage)
			d.Content.Stats.TotalUsageResetAt = time.Now().UnixMilli()
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		return c.JSON(http.StatusOK, makeStatsResponse(d))
	}
}
