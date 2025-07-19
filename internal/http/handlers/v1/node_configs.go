package v1

import (
	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/writer"
	"net/http"
	"strconv"
	"time"
)

func NodesConfigsShow(cdr *coordinator.Coordinator, writer *writer.Writer, d *database.Database) echo.HandlerFunc {
	return func(c echo.Context) error {
		d.Locker.Lock()
		defer d.Locker.Unlock()

		nodeId := c.Param("id")
		var node *database.Node
		for _, n := range d.Content.Nodes {
			if strconv.Itoa(n.Id) == nodeId {
				node = n
				node.PulledAt = time.Now().UnixMilli()
				node.PullStatus = database.NodeStatusAvailable

				if err := d.Save(); err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if node == nil {
			return c.NoContent(http.StatusNotFound)
		}

		configs := writer.RemoteConfig(node, cdr.State().XrayUpdatedAt(), cdr.State().XraySharedPassword())

		return c.JSON(http.StatusOK, configs)
	}
}
