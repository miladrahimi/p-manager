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

// defaultConfigPath returns the path of the default config file.
func defaultConfigPath(root string) string {
	return filepath.Join(root, "configs/main.defaults.json")
}

// localConfigPath returns the path of the optional local config file.
func localConfigPath(root string) string {
	return filepath.Join(root, "configs/main.json")
}

// SshStdoutPath returns the path of the log file for ssh standard output.
func SshStdoutPath(root string) string {
	return filepath.Join(root, "storage/logs/ssh-out.log")
}

// SshStderrPath returns the path of the log file for ssh standard error.
func SshStderrPath(root string) string {
	return filepath.Join(root, "storage/logs/ssh-err.log")
}

// XrayConfigPath returns the path of the xray config file.
func XrayConfigPath(root string) string {
	return filepath.Join(root, "storage/app/xray.json")
}

// XrayBinaryPath returns the path of the xray binary for the current OS.
func XrayBinaryPath(root string) string {
	path, found := xrayBinaryPaths[runtime.GOOS]
	if !found {
		path = xrayBinaryPaths["linux"]
	}
	return filepath.Join(root, path)
}
