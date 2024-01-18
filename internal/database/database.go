package database

import (
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/labstack/gommon/random"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

const Path = "storage/database.json"

type Data struct {
	Settings *Settings `json:"settings"`
	Stats    *Stats    `json:"stats"`
	Users    []*User   `json:"users"`
	Servers  []*Server `json:"servers"`
}

type Database struct {
	Data   *Data
	Locker *sync.Mutex
	log    *zap.Logger
}

func (d *Database) Init() {
	if !utils.FileExist(Path) {
		d.Save()
	}
	d.Load()
}

func (d *Database) Load() {
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
		d.Load()
	}()
	content, err := json.Marshal(d.Data)
	if err != nil {
		d.log.Fatal("database: cannot marshal data", zap.Error(err))
	}

	if err = os.WriteFile(Path, content, 0755); err != nil {
		d.log.Fatal("database: cannot save file", zap.String("file", Path), zap.Error(err))
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

func New(l *zap.Logger) *Database {
	return &Database{
		log:    l,
		Locker: &sync.Mutex{},
		Data: &Data{
			Settings: &Settings{
				AdminPassword: "password",
				Host:          "127.0.0.1",
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
