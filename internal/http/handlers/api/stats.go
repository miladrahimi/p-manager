package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
)

type StatsUpdatePartialRequest struct {
	TotalUsage *float64 `json:"total_usage" validate:"omitempty,min=0"`
}

type StatsResponse struct {
	TotalUsageResetAt int64   `json:"total_usage_reset_at"`
	TotalUsage        float64 `json:"total_usage"`
	TotalUsers        int     `json:"total_users"`
	ActiveUsers       int     `json:"active_users"`
}

func makeStatsResponse(d *database.Database[data.Data]) *StatsResponse {
	db := d.Data()
	return &StatsResponse{
		TotalUsageResetAt: db.Stats.TotalUsageResetAt,
		TotalUsage:        db.Stats.TotalUsage,
		TotalUsers:        len(db.Users),
		ActiveUsers:       db.CountActiveUsers(),
	}
}

func StatsIndex(d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, makeStatsResponse(d))
	}
}

func StatsUpdatePartial(d *database.Database[data.Data]) echo.HandlerFunc {
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

		if request.TotalUsage != nil {
			db := d.Data()
			db.Stats.TotalUsage = *request.TotalUsage
			db.Stats.TotalUsageBytes = util.GB2Bytes(*request.TotalUsage)
			db.Stats.TotalUsageResetAt = time.Now().UnixMilli()
		}

		if err := d.Save(); err != nil {
			return errors.WithStack(err)
		}

		return c.JSON(http.StatusOK, makeStatsResponse(d))
	}
}
