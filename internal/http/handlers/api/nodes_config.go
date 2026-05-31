package api

import (
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
)

// NodesConfigShow shows a single node configuration required for P-Node pulling.
func NodesConfigShow(
	cdr *coordinator.Coordinator,
	composer *composer.Composer,
	db *data.Store,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		nodeId := c.Param("id")

		var node *data.Node
		err := db.Mutate(func(d *data.Data) (bool, error) {
			for _, n := range d.Nodes {
				if n.Id == nodeId {
					node = n
					node.PulledAt = time.Now().UnixMilli()
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if node == nil {
			return c.NoContent(http.StatusNotFound)
		}

		nc := composer.NodeConfig(node, cdr.State().XrayUpdatedAt())

		return c.JSON(http.StatusOK, nc)
	}
}
