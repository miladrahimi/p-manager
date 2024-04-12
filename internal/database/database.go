package database

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator"
	"github.com/labstack/gommon/random"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"time"
)

const Path = "storage/database/app.json"
const BackupPath = "storage/database/backup-%s.json"

type Data struct {
	Settings *Settings `json:"settings"`
	Stats    *Stats    `json:"stats"`
	Users    []*User   `json:"users"`
	Servers  []*Server `json:"servers"`
}

type Database struct {
	Data   *Data
	Locker *sync.Mutex
	l      *logger.Logger
}

func (d *Database) Init() {
	d.Locker.Lock()
	defer d.Locker.Unlock()

	if !utils.FileExist(Path) {
		d.Save()
	} else {
		d.Load()
	}
}

func (d *Database) Load() {
	content, err := os.ReadFile(Path)
	if err != nil {
		d.l.Fatal("cannot read file", zap.Error(errors.WithStack(err)))
	}

	err = json.Unmarshal(content, d.Data)
	if err != nil {
		d.l.Fatal("cannot unmarshal data", zap.Error(errors.WithStack(err)))
	}

	if err = validator.New().Struct(d); err != nil {
		d.l.Fatal("cannot validate data", zap.Error(errors.WithStack(err)))
	}
}

func (d *Database) Save() {
	content, err := json.Marshal(d.Data)
	if err != nil {
		d.l.Fatal("cannot marshal data", zap.Error(errors.WithStack(err)))
	}

	if err = os.WriteFile(Path, content, 0755); err != nil {
		d.l.Fatal("cannot save data", zap.Error(errors.WithStack(err)))
	}
}

func (d *Database) Backup() {
	d.Locker.Lock()
	defer d.Locker.Unlock()

	content, err := json.Marshal(d.Data)
	if err != nil {
		d.l.Error("database: cannot marshal data", zap.Error(errors.WithStack(err)))
	}

	path := strings.ToLower(fmt.Sprintf(BackupPath, time.Now().Format("Mon-15")))
	if err = os.WriteFile(path, content, 0755); err != nil {
		d.l.Fatal("cannot save backup file", zap.String("file", path), zap.Error(errors.WithStack(err)))
	}
}

func (d *Database) CountActiveUsers() int {
	activeUsersCount := len(d.Data.Users)
	for _, u := range d.Data.Users {
		if !u.Enabled {
			activeUsersCount--
		}
	}
	return activeUsersCount
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
		Locker: &sync.Mutex{},
		l:      l,
		Data: &Data{
			Settings: &Settings{
				AdminPassword: "password",
				Host:          "127.0.0.1",
				SsReversePort: 0,
				SsRelayPort:   0,
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
