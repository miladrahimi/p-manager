package licensor

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/enigma"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/utils"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
	"net/http"
	"os"
)

type Licensor struct {
	l        *logger.Logger
	c        *config.Config
	hc       *client.Client
	database *database.Database
	enigma   *enigma.Enigma
	licensed bool
}

func (l *Licensor) Run() {
	go func() {
		l.fetch()

		if err := l.validate(); err != nil {
			l.l.Error("licensor: cannot validate license", zap.Error(errors.WithStack(err)))
		}
	}()
}

func (l *Licensor) fetch() {
	body := map[string]interface{}{
		"host": l.database.Content.Settings.Host,
		"port": l.c.HttpServer.Port,
	}
	if r, err := l.hc.Do(http.MethodPost, config.LicenseServer, config.LicenseToken, body); err != nil {
		l.l.Debug("licensor: cannot fetch license", zap.Error(errors.WithStack(err)))
	} else {
		var response map[string]string
		if err = json.Unmarshal(r, &response); err != nil {
			l.l.Debug("licensor: cannot unmarshall server response", zap.Error(errors.WithStack(err)))
		}
		if license, found := response["license"]; found {
			if err = os.WriteFile(l.c.Env.LicensePath, []byte(license), 0755); err != nil {
				l.l.Debug("licensor: cannot save license file", zap.Error(errors.WithStack(err)))
			}
		} else {
			l.l.Debug("licensor: license is not issued")
		}
	}
}

func (l *Licensor) validate() error {
	if !utils.FileExist(l.c.Env.LicensePath) {
		l.l.Debug("licensor: no license file found")
		return nil
	}

	licenseFile, err := os.ReadFile(l.c.Env.LicensePath)
	if err != nil {
		return errors.WithStack(err)
	}

	key := fmt.Sprintf("%s:%d", l.database.Content.Settings.Host, l.c.HttpServer.Port)
	l.licensed = l.enigma.Verify(key, string(licenseFile))
	l.l.Info("licensor: license file checked", zap.Bool("valid", l.licensed))

	return nil
}

func (l *Licensor) Licensed() bool {
	return l.licensed
}

func New(
	config *config.Config,
	hc *client.Client,
	logger *logger.Logger,
	database *database.Database,
	enigma *enigma.Enigma,
) *Licensor {
	return &Licensor{
		l:        logger,
		hc:       hc,
		c:        config,
		database: database,
		enigma:   enigma,
		licensed: false,
	}
}
