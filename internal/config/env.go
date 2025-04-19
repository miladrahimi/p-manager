package config

import (
	"path/filepath"
	"runtime"
)

var xrayBinaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
}

type Env struct {
	AppDirectory       string
	LicensePath        string
	EnigmaKeyPath      string
	XrayConfigPath     string
	XrayBinaryPath     string
	DefaultConfigPath  string
	LocalConfigPath    string
	DatabasePath       string
	DatabaseBackupPath string
}

func NewEnv(appDirectory string) *Env {
	xrayBinaryPath := filepath.Join(appDirectory, xrayBinaryPaths["linux"])
	if path, found := xrayBinaryPaths[runtime.GOOS]; found {
		xrayBinaryPath = filepath.Join(appDirectory, path)
	}

	return &Env{
		AppDirectory:       appDirectory,
		XrayBinaryPath:     xrayBinaryPath,
		DefaultConfigPath:  filepath.Join(appDirectory, "configs/main.defaults.json"),
		LocalConfigPath:    filepath.Join(appDirectory, "configs/main.json"),
		LicensePath:        filepath.Join(appDirectory, "storage/app/license.txt"),
		EnigmaKeyPath:      filepath.Join(appDirectory, "resources/ed25519_public_key.txt"),
		XrayConfigPath:     filepath.Join(appDirectory, "storage/app/xray.json"),
		DatabasePath:       filepath.Join(appDirectory, "storage/database/app.json"),
		DatabaseBackupPath: filepath.Join(appDirectory, "storage/database/backup-%s.json"),
	}
}
