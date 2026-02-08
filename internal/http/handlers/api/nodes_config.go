package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
)

func NodesConfigsShow(cdr *coordinator.Coordinator, writer *composer.Composer, d *database.Database[data.Data]) echo.HandlerFunc {
	return func(c echo.Context) error {
		nodeId := c.Param("id")
		var node *data.Node
		for _, n := range d.Data().Nodes {
			if strconv.Itoa(n.Id) == nodeId {
				node = n
				node.PulledAt = time.Now().UnixMilli()
				node.PullStatus = data.NodeStatusAvailable

				if err := d.Save(); err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if node == nil {
			return c.NoContent(http.StatusNotFound)
		}

		configs := writer.NodeConfig(node, cdr.State().XrayUpdatedAt(), cdr.State().XraySharedPassword())

		return c.JSON(http.StatusOK, configs)
	}
}
