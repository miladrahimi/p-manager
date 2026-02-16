package composer

import (
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/xray"
)

// Composer composes the xray config.
type Composer struct {
	config *config.Config
	db     *database.Database[data.Data]
	xray   *xray.Xray
}

// New creates a new composer.
func New(config *config.Config, database *database.Database[data.Data], xray *xray.Xray) *Composer {
	return &Composer{config: config, db: database, xray: xray}
}

