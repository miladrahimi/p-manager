package database

import (
	"encoding/json"
	"github.com/go-playground/validator"
	"github.com/labstack/gommon/random"
	"go.uber.org/zap"
	"os"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/utils"
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
	Data *Data
	log  *zap.Logger
	lock sync.Mutex
}

func (d *Database) Init() {
	if !utils.FileExist(Path) {
		d.Save()
	}
	d.Load()
}

func (d *Database) Load() {
	d.lock.Lock()
	defer d.lock.Unlock()

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

	d.lock.Lock()
	defer d.lock.Unlock()

	if err = os.WriteFile(Path, content, 0755); err != nil {
		d.log.Fatal("database: cannot save file", zap.String("file", Path), zap.Error(err))
	}
}

func (d *Database) GenerateUserId() int {
	d.lock.Lock()
	defer d.lock.Unlock()

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
	d.lock.Lock()
	defer d.lock.Unlock()

	for {
		r := random.String(16)
		isUnique := true
		for _, user := range d.Data.Users {
			if user.Password == r {
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
	d.lock.Lock()
	defer d.lock.Unlock()

	if len(d.Data.Servers) > 0 {
		return d.Data.Servers[len(d.Data.Servers)-1].Id + 1
	} else {
		return 1
	}
}

func New(l *zap.Logger) *Database {
	return &Database{
		log: l,
		Data: &Data{
			Settings: &Settings{
				AdminPassword:   "password",
				ShadowsocksHost: "127.0.0.1",
				ShadowsocksPort: 1919,
				HttpsAddress:    "",
				HttpAddress:     "http://127.0.0.1",
				TrafficRatio:    1,
			},
			Stats: &Stats{
				Inbound:   0,
				Outbound:  0,
				UpdatedAt: time.Now().UnixMilli(),
			},
			Users: []*User{
				{
					Id:        1,
					Identity:  utils.UUID(),
					Name:      "user1",
					Password:  "password",
					Method:    config.ShadowsocksMethod,
					Quota:     0,
					Used:      0,
					Enabled:   true,
					CreatedAt: time.Now().UnixMilli(),
				},
			},
			Servers: []*Server{
				{
					Id:       1,
					Host:     "127.0.0.1",
					Port:     1919,
					Password: "password",
					Method:   config.ShadowsocksMethod,
					Status:   ServerStatusAvailable,
				},
			},
		},
	}
}
