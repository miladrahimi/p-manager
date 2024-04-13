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

const Server = "https://x.miladrahimi.com/p-manager/v1/servers"
const Token = "Unauthorized"

type Licensor struct {
	l        *logger.Logger
	config   *config.Config
	database *database.Database
	hc       *client.Client
	enigma   *enigma.Enigma
	licensed bool
}

func (l *Licensor) Init() {
	go func() {
		l.fetch()
		l.validate()
	}()
}

func (l *Licensor) validate() {
	if !utils.FileExist(config.LicensePath) {
		l.l.Debug("licensor: no license file found")
	} else {
		licenseFile, err := os.ReadFile(config.LicensePath)
		if err != nil {
			l.l.Error("licensor: cannot read license file", zap.Error(errors.WithStack(err)))
		} else {
			key := fmt.Sprintf("%s:%d", l.database.Data.Settings.Host, l.config.HttpServer.Port)
			l.licensed = l.enigma.Verify(key, string(licenseFile))
			l.l.Info("licensor: license file checked", zap.Bool("valid", l.licensed))
		}
	}
}

func (l *Licensor) fetch() {
	body := map[string]interface{}{
		"host": l.database.Data.Settings.Host,
		"port": l.config.HttpServer.Port,
	}
	if r, err := l.hc.Do(http.MethodPost, Server, Token, body); err != nil {
		l.l.Debug("licensor: cannot fetch license", zap.Error(errors.WithStack(err)))
	} else {
		var response map[string]string
		if err = json.Unmarshal(r, &response); err != nil {
			l.l.Debug("licensor: cannot unmarshall server response", zap.Error(errors.WithStack(err)))
		}
		if license, found := response["license"]; found {
			if err = os.WriteFile(config.LicensePath, []byte(license), 0755); err != nil {
				l.l.Debug("licensor: cannot save license file", zap.Error(errors.WithStack(err)))
			}
		} else {
			l.l.Debug("licensor: license is not issued")
		}
	}
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
		config:   config,
		database: database,
		enigma:   enigma,
		licensed: false,
	}
}
