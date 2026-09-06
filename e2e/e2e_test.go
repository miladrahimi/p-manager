//go:build e2e

// Package e2e runs P-Manager and P-Node as real processes and checks that an
// account can reach the internet through every supported proxy method.
//
// Run with: make e2e (go test -tags e2e ./e2e/). Set E2E_SSH=1 to include
// Relay RR2SSH, which needs passwordless ssh to 127.0.0.1 for the current user.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	adminPassword = "password" // default admin password (internal/data/main_settings.go)
	sshPort       = 22
)

var xrayDirs = map[string]string{
	"darwin": "third_party/xray-macos-arm64",
	"linux":  "third_party/xray-linux-64",
}

func TestProxyMethods(t *testing.T) {
	managerRepo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	nodeRepo := os.Getenv("P_NODE_DIR")
	if nodeRepo == "" {
		nodeRepo = filepath.Join(managerRepo, "..", "p-node")
	}
	withSsh := os.Getenv("E2E_SSH") == "1"

	xrayDir, ok := xrayDirs[runtime.GOOS]
	if !ok {
		t.Skipf("no xray binary for %s", runtime.GOOS)
	}
	xrayBinary := filepath.Join(managerRepo, xrayDir, "xray")
	if _, err := os.Stat(xrayBinary); err != nil {
		t.Skipf("xray binary not found at %s (run make local-setup)", xrayBinary)
	}

	// The target the proxied requests must reach.
	const body = "p-manager e2e ok"
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer target.Close()

	work := t.TempDir()
	managerBin := build(t, managerRepo, filepath.Join(work, "p-manager"))
	nodeBin := build(t, nodeRepo, filepath.Join(work, "p-node"))

	ports := freePorts(t, 9)
	managerPort, nodePort := ports[0], ports[1]
	directRr, remoteRr, relayRr2RrManager, relayRr2RrNode, reverseRr, relayRr2Ssh := ports[2], ports[3], ports[4], ports[5], ports[6], ports[7]
	if !withSsh {
		relayRr2Ssh = 0
	}

	// --- P-Node ---
	nodeRoot := prepareRoot(t, nodeRepo, filepath.Join(work, "node"), xrayDir, map[string]any{
		"http_server": map[string]any{"port": nodePort},
		"logger":      map[string]any{"level": "debug", "format": "2006-01-02 15:04:05.000"},
	})
	nodeProc := startProcess(t, nodeBin, nodeRoot)
	nodeUrl := fmt.Sprintf("http://127.0.0.1:%d", nodePort)
	waitHttp(t, nodeUrl+"/", "p-node")

	var nodeDb struct {
		Settings struct {
			HttpToken string `json:"http_token"`
		} `json:"settings"`
	}
	readJsonFile(t, filepath.Join(nodeRoot, "storage/database/data.json"), &nodeDb)

	// --- P-Manager ---
	managerRoot := prepareRoot(t, managerRepo, filepath.Join(work, "manager"), xrayDir, map[string]any{
		"http_server": map[string]any{"host": "127.0.0.1", "port": managerPort},
		"logger":      map[string]any{"level": "debug", "format": "2006-01-02 15:04:05.000"},
	})
	managerProc := startProcess(t, managerBin, managerRoot)
	managerUrl := fmt.Sprintf("http://127.0.0.1:%d", managerPort)
	waitHttp(t, managerUrl+"/api/admin/platform", "p-manager")

	t.Cleanup(func() {
		if t.Failed() {
			dumpLogs(t, "manager", managerRoot)
			dumpLogs(t, "node", nodeRoot)
		}
		managerProc.stop()
		nodeProc.stop()
	})

	admin := &apiClient{base: managerUrl + "/api/admin", token: adminPassword}

	// Xray settings: keep the generated Reality keys, enable every method.
	var xs map[string]any
	admin.call(t, http.MethodGet, "/xray-settings", nil, &xs)
	if xs["reality_private_key"] == "" {
		t.Fatal("manager did not generate reality keys")
	}
	xs["direct_rr_port"] = directRr
	xs["remote_rr_port"] = remoteRr
	xs["relay_rr_2_rr_manager_port"] = relayRr2RrManager
	xs["relay_rr_2_rr_node_port"] = relayRr2RrNode
	xs["reverse_rr_manager_port"] = reverseRr
	xs["relay_rr_2_ssh_port"] = relayRr2Ssh
	xs["relay_rr_2_ssh_connections"] = 1
	admin.call(t, http.MethodPost, "/xray-settings", xs, nil)

	// Node: pushed over HTTP; SSH only when the environment provides it.
	sshUser := "root"
	if u, err := user.Current(); err == nil {
		sshUser = u.Username
	}
	var node struct {
		Id        string `json:"id"`
		PullToken string `json:"pull_token"`
	}
	admin.call(t, http.MethodPost, "/nodes", map[string]any{
		"host":         "127.0.0.1",
		"http_token":   nodeDb.Settings.HttpToken,
		"http_port":    nodePort,
		"ssh_user":     sshUser,
		"ssh_port":     sshPort,
		"ssh_enabled":  withSsh,
		"push_enabled": true,
	}, &node)

	// Node pulls too, so both sync paths are exercised.
	nodeApi := &apiClient{base: nodeUrl, token: nodeDb.Settings.HttpToken}
	nodeApi.call(t, http.MethodPost, "/manager", map[string]any{
		"url":   fmt.Sprintf("%s/api/node/%s", managerUrl, node.Id),
		"token": node.PullToken,
	}, nil)

	var account struct {
		Id string `json:"id"`
	}
	admin.call(t, http.MethodPost, "/accounts", map[string]any{
		"name": "e2e", "enabled": true, "quota": 0,
	}, &account)

	// Wait until the node runs the account's inbound, i.e. the config arrived.
	waitFor(t, 60*time.Second, "node to receive the remote-rr inbound", func() bool {
		var xc struct {
			Inbounds []struct {
				Tag string `json:"tag"`
			} `json:"inbounds"`
		}
		if err := readJson(filepath.Join(nodeRoot, "storage/app/xray.json"), &xc); err != nil {
			return false
		}
		for _, in := range xc.Inbounds {
			if in.Tag == "remote-rr" {
				return true
			}
		}
		return false
	})

	var shown struct {
		Proxies map[string]string `json:"proxies"`
	}
	(&apiClient{base: managerUrl + "/api/user"}).call(t, http.MethodGet, "/account/"+account.Id, nil, &shown)

	want := []string{"direct-rr", "remote-rr-127.0.0.1", "relay-rr2rr", "reverse-rr"}
	if withSsh {
		want = append(want, "relay-rr2ssh")
	}
	for _, method := range want {
		method := method
		link, ok := shown.Proxies[method+"@127.0.0.1"]
		if !ok {
			t.Errorf("no %s link in %v", method, keys(shown.Proxies))
			continue
		}
		t.Run(method, func(t *testing.T) {
			got := fetchThroughLink(t, xrayBinary, filepath.Join(work, "client-"+method), link, target.URL)
			if got != body {
				t.Fatalf("got %q through %s, want %q", got, method, body)
			}
		})
	}
	if !withSsh {
		t.Log("relay-rr2ssh skipped; set E2E_SSH=1 to include it")
	}
}

// fetchThroughLink runs an Xray client for the share link and fetches the target through it.
func fetchThroughLink(t *testing.T, xrayBinary, dir, link, targetUrl string) string {
	t.Helper()

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("bad link %q: %v", link, err)
	}
	port, _ := strconv.Atoi(u.Port())
	q := u.Query()
	httpPort := freePorts(t, 1)[0]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"tag": "in", "protocol": "http", "listen": "127.0.0.1", "port": httpPort, "settings": map[string]any{},
		}},
		"outbounds": []any{map[string]any{
			"tag":      "out",
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": u.Hostname(), "port": port,
				"users": []any{map[string]any{
					"id": u.User.Username(), "flow": q.Get("flow"), "encryption": "none",
				}},
			}}},
			"streamSettings": map[string]any{
				"network":  q.Get("type"),
				"security": q.Get("security"),
				"realitySettings": map[string]any{
					"serverName": q.Get("sni"), "publicKey": q.Get("pbk"), "shortId": "", "fingerprint": "chrome",
				},
			},
		}},
	}
	configPath := filepath.Join(dir, "config.json")
	writeJson(t, configPath, config)

	proc := startProcess(t, xrayBinary, dir, "-c", configPath)
	defer proc.stop()
	waitPort(t, httpPort, "xray client")

	proxyUrl, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
	}

	// The chain may still be settling (reverse bridges register asynchronously).
	var lastErr error
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(targetUrl)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return string(b)
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, b)
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("request through proxy failed: %v", lastErr)
	return ""
}

// --- helpers ---

type apiClient struct {
	base  string
	token string
}

func (c *apiClient) call(t *testing.T, method, path string, in, out any) {
	t.Helper()
	var reader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d: %s", method, path, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: bad json %q: %v", method, path, b, err)
		}
	}
}

// prepareRoot lays out a working directory the way the app expects it under its cwd.
func prepareRoot(t *testing.T, repo, root, xrayDir string, localConfig map[string]any) string {
	t.Helper()
	for _, d := range []string{"configs", "storage/app", "storage/database", "storage/logs", filepath.Dir(xrayDir)} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(t, filepath.Join(repo, "configs/main.defaults.json"), filepath.Join(root, "configs/main.defaults.json"))
	writeJson(t, filepath.Join(root, "configs/main.json"), localConfig)
	if err := os.Symlink(filepath.Join(repo, xrayDir), filepath.Join(root, xrayDir)); err != nil {
		t.Fatal(err)
	}
	return root
}

func build(t *testing.T, repo, out string) string {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = repo
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", repo, err, b)
	}
	return out
}

type process struct {
	cmd *exec.Cmd
}

func startProcess(t *testing.T, bin, dir string, args ...string) *process {
	t.Helper()
	if len(args) == 0 {
		args = []string{"serve"}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	logFile, err := os.Create(filepath.Join(dir, "stdout.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", bin, err)
	}
	return &process{cmd: cmd}
}

func (p *process) stop() {
	if p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _, _ = p.cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
	}
}

func waitHttp(t *testing.T, url, what string) {
	t.Helper()
	waitFor(t, 30*time.Second, what+" to answer at "+url, func() bool {
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	})
}

func waitPort(t *testing.T, port int, what string) {
	t.Helper()
	waitFor(t, 15*time.Second, fmt.Sprintf("%s to listen on %d", what, port), func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	})
}

func waitFor(t *testing.T, timeout time.Duration, what string, ready func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func freePorts(t *testing.T, n int) []int {
	t.Helper()
	ports := make([]int, 0, n)
	listeners := make([]net.Listener, 0, n)
	for len(ports) < n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	for _, l := range listeners {
		_ = l.Close()
	}
	return ports
}

func readJsonFile(t *testing.T, path string, out any) {
	t.Helper()
	if err := readJson(path, out); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
}

func readJson(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func writeJson(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func dumpLogs(t *testing.T, name, root string) {
	for _, f := range []string{"stdout.log", "storage/logs/app-std.log", "storage/logs/xray-error.log"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil || len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) > 60 {
			lines = lines[len(lines)-60:]
		}
		t.Logf("--- %s %s (last %d lines) ---\n%s", name, f, len(lines), strings.Join(lines, "\n"))
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
