package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/provisioner"
)

type NodesHetznerShowResponse struct {
	// Configured is whether a Hetzner token is set (provisioning is possible).
	Configured bool              `json:"configured"`
	Jobs       []provisioner.Job `json:"jobs"`
}

type NodesHetznerStoreRequest struct {
	// ManagerUrl is the public base URL of this P-Manager, used to configure
	// pulling on the new node. Optional.
	ManagerUrl string `json:"manager_url" validate:"omitempty,url,max=1024"`
}

// NodesHetznerShow returns the Hetzner provisioning state (jobs).
func NodesHetznerShow(p *provisioner.Provisioner) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, NodesHetznerShowResponse{
			Configured: p.Configured(),
			Jobs:       p.Jobs(),
		})
	}
}

// NodesHetznerStore starts provisioning a new node on Hetzner in the background.
func NodesHetznerStore(p *provisioner.Provisioner) echo.HandlerFunc {
	return func(c echo.Context) error {
		var r NodesHetznerStoreRequest
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

		job, err := p.Start(r.ManagerUrl)
		switch {
		case errors.Is(err, provisioner.ErrTokenMissing):
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Set the Hetzner API token in the main settings first."})
		case errors.Is(err, provisioner.ErrBusy):
			return c.JSON(http.StatusConflict, map[string]string{"message": "A node is already being provisioned."})
		case errors.Is(err, provisioner.ErrMaxNodes):
			return c.JSON(http.StatusForbidden, map[string]string{"message": "Cannot add more nodes!"})
		case err != nil:
			return errors.WithStack(err)
		}

		return c.JSON(http.StatusAccepted, job)
	}
}

// NodesHetznerDismiss removes a failed provisioning job from the list.
func NodesHetznerDismiss(p *provisioner.Provisioner) echo.HandlerFunc {
	return func(c echo.Context) error {
		p.Dismiss(c.Param("jobId"))
		return c.NoContent(http.StatusNoContent)
	}
}
