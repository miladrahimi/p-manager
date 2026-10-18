package config

import (
	"path/filepath"
	"runtime"
)

// Binary paths of the embedded third-party tools, keyed by GOOS/GOARCH. Linux
// amd64 is the production platform; the others serve local development and e2e.
var xrayBinaryPaths = map[string]string{
	"darwin/arm64": "third_party/xray-macos-arm64/xray",
	"linux/amd64":  "third_party/xray-linux-64/xray",
	"linux/arm64":  "third_party/xray-linux-arm64/xray",
}

var paqetBinaryPaths = map[string]string{
	"darwin/arm64": "third_party/paqet-macos-arm64/paqet",
	"linux/amd64":  "third_party/paqet-linux-64/paqet",
	"linux/arm64":  "third_party/paqet-linux-arm64/paqet",
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

// PaqetConfigPath returns the path of the paqet client config file for a node.
func PaqetConfigPath(root, nodeId string) string {
	return filepath.Join(root, "storage/app/paqet-"+nodeId+".yaml")
}

// PaqetLogPath returns the path of the log file shared by the paqet clients.
func PaqetLogPath(root string) string {
	return filepath.Join(root, "storage/logs/paqet.log")
}

// XrayBinaryPath returns the path of the xray binary for the current platform.
func XrayBinaryPath(root string) string {
	return filepath.Join(root, binaryPath(xrayBinaryPaths))
}

// PaqetBinaryPath returns the path of the paqet binary for the current platform.
func PaqetBinaryPath(root string) string {
	return filepath.Join(root, binaryPath(paqetBinaryPaths))
}

func binaryPath(paths map[string]string) string {
	path, found := paths[runtime.GOOS+"/"+runtime.GOARCH]
	if !found {
		path = paths["linux/amd64"]
	}
	return path
}

// LogsDirectory returns the path of the directory holding all log files
// (app, ssh, xray, paqet).
func LogsDirectory(root string) string {
	return filepath.Join(root, "storage/logs")
}
