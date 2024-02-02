package database

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator"
	"github.com/labstack/gommon/random"
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"time"
)

const Path = "storage/database.json"
const BackupPath = "storage/backup-%s.json"

type Data struct {
	Settings *Settings `json:"settings"`
	Stats    *Stats    `json:"stats"`
	Users    []*User   `json:"users"`
	Servers  []*Server `json:"servers"`
}

type Database struct {
	Data   *Data
	locker *sync.Mutex
	log    *logger.Logger
}

func (d *Database) Init() {
	if !utils.FileExist(Path) {
		d.Save()
	}
	d.Load()
}

func (d *Database) Load() {
	d.locker.Lock()
	defer d.locker.Unlock()

	content, err := os.ReadFile(Path)
	if err != nil {
		d.log.Fatal("database: cannot load file", zap.String("file", Path), zap.Error(err))
	}

	err = json.Unmarshal(content, d.Data)
	if err != nil {
		d.log.Fatal("database: cannot unmarshall data", zap.Error(err))
	}

	if err = validator.New().Struct(d); err != nil {
		d.log.Fatal("database: cannot validate data", zap.Error(err))
	}
}

func (d *Database) Save() {
	defer func() {
		d.locker.Unlock()
		d.Load()
	}()
	d.locker.Lock()

	content, err := json.Marshal(d.Data)
	if err != nil {
		d.log.Fatal("database: cannot marshal data", zap.Error(err))
	}

	if err = os.WriteFile(Path, content, 0755); err != nil {
		d.log.Fatal("database: cannot save file", zap.String("file", Path), zap.Error(err))
	}
}

func (d *Database) Backup() {
	content, err := json.Marshal(d.Data)
	if err != nil {
		d.log.Error("database: cannot marshal data", zap.Error(err))
	}

	path := strings.ToLower(fmt.Sprintf(BackupPath, time.Now().Format("Mon-15")))
	if err = os.WriteFile(path, content, 0755); err != nil {
		d.log.Fatal("database: cannot save backup file", zap.String("file", path), zap.Error(err))
	}
}

func (d *Database) GenerateUserId() int {
	if len(d.Data.Users) > 0 {
		return d.Data.Users[len(d.Data.Users)-1].Id + 1
	} else {
		return 1
	}
}

func (d *Database) GenerateUserIdentity() string {
	return utils.UUID()
}

func (d *Database) GenerateUserPassword() string {
	for {
		r := random.String(16)
		isUnique := true
		for _, user := range d.Data.Users {
			if user.ShadowsocksPassword == r {
				isUnique = false
				break
			}
		}
		if isUnique {
			return r
		}
	}
}

func (d *Database) GenerateServerId() int {
	if len(d.Data.Servers) > 0 {
		return d.Data.Servers[len(d.Data.Servers)-1].Id + 1
	} else {
		return 1
	}
}

func New(l *logger.Logger) *Database {
	return &Database{
		locker: &sync.Mutex{},
		log:    l,
		Data: &Data{
			Settings: &Settings{
				AdminPassword: "password",
				Host:          "127.0.0.1",
				SsReversePort: 1,
				SsRelayPort:   1,
				TrafficRatio:  1,
			},
			Stats: &Stats{
				Traffic:   0,
				UpdatedAt: time.Now().UnixMilli(),
			},
			Users:   []*User{},
			Servers: []*Server{},
		},
	}
}
