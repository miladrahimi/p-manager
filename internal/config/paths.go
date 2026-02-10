package config

import (
	"path/filepath"
	"runtime"
)

// XrayBinaryPath returns the path of the xray binary for the current OS.
var xrayBinaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
}

// DatabaseDirectory returns the path of the database directory.
func DatabaseDirectory(root string) string {
	return filepath.Join(root, "storage/database")
}

// XrayConfigPath returns the path of the xray config file.
func XrayConfigPath(root string) string {
	return filepath.Join(root, "storage/app/xray.json")
}

// defaultConfigPath returns the path of the default config file.
func defaultConfigPath(root string) string {
	return filepath.Join(root, "configs/main.defaults.json")
}

// localConfigPath returns the path of the optional local config file.
func localConfigPath(root string) string {
	return filepath.Join(root, "configs/main.json")
}

// XrayBinaryPath returns the path of the xray binary for the current OS.
func XrayBinaryPath(root string) string {
	path, found := xrayBinaryPaths[runtime.GOOS]
	if !found {
		path = xrayBinaryPaths["linux"]
	}
	return filepath.Join(root, path)
}
